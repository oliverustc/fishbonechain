package gnarkwrapper

import (
	"encoding/hex"
	"fmt"
	"gnarkabc/logger"
	"gnarkabc/utils"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/consensys/gnark-crypto/ecc"
	fr_bn254 "github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

func (g *Groth16Wrapper) ExportSolidity(filePath string) {
	if filePath == "" {
		logger.Info("filePath is empty, using default path: output/Groth16Verifier.sol")
		filePath = "output/Groth16Verifier.sol"
	}
	if g.Curve != ecc.BN254 {
		logger.Fatal("only BN254 curve is supported")
	}
	utils.EnsureDirExists("output")
	solFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		logger.Fatal("open file failed: %v", err)
	}
	defer solFile.Close()
	g.VK.ExportSolidity(solFile)
	logger.Info("export solidity to %s", filePath)
}

// 参与gnark-solidity-checker验证流程的准备工作，将proof序列化
func (g *Groth16Wrapper) ProofMarshall() (proofStr string) {
	if g.Curve != ecc.BN254 {
		logger.Fatal("only BN254 curve is supported")
	}

	_proof, ok := g.Proof.(interface{ MarshalSolidity() []byte })
	if !ok {
		logger.Fatal("marshal proof failed")
	}
	proofStr = hex.EncodeToString(_proof.MarshalSolidity())
	return
}

// 参与gnark-solidity-checker验证流程的准备工作，将public witness序列化
func (g *Groth16Wrapper) PublicWitnessMarshall() (publicWitnessStr string) {
	if g.Curve != ecc.BN254 {
		logger.Fatal("only BN254 curve is supported")
	}
	bPublicWitness, err := g.WitnessPublic.MarshalBinary()
	if err != nil {
		logger.Fatal("marshal public witness failed: %v", err)
	}
	bPublicWitness = bPublicWitness[12:]
	publicWitnessStr = hex.EncodeToString(bPublicWitness)
	return
}

// 参与gnark-solidity-checker验证流程的准备工作，获取public input的长度
func (g *Groth16Wrapper) GetPublicInputNum() string {
	if g.Curve != ecc.BN254 {
		logger.Fatal("only BN254 curve is supported")
	}
	return strconv.Itoa(len(g.WitnessPublic.Vector().(fr_bn254.Vector)))
}

// 阅读gnark-solidity-checker的源码，简化下的proof的处理流程
func (g *Groth16Wrapper) GenSolProofParams() string {
	if g.Curve != ecc.BN254 {
		logger.Fatal("only BN254 curve is supported")
	}

	_proof, ok := g.Proof.(interface{ MarshalSolidity() []byte })
	if !ok {
		logger.Fatal("marshal proof failed")
	}
	proofBytes := _proof.MarshalSolidity()
	logger.Info("length of proof: %d", len(proofBytes))
	var proof [8]*big.Int
	for i := range 8 {
		proof[i] = new(big.Int).SetBytes(proofBytes[FpSize*i : FpSize*(i+1)])
	}

	proofStr := "["
	for i := range proof {
		proofStr += proof[i].String() + ","
	}
	proofStr = proofStr[:len(proofStr)-1]
	proofStr += "]"
	commitmentCount := g.GenNbCommitments()
	// 判断proof中是否有commitments
	if commitmentCount > 0 {
		logger.Info("proof has commitments")
		var commitments [2]*big.Int
		var commitmentPok [2]*big.Int

		// commitments
		for i := 0; i < 2*commitmentCount; i++ {
			commitments[i] = new(big.Int).SetBytes(proofBytes[FpSize*8+4+i*FpSize : FpSize*8+4+(i+1)*FpSize])
		}

		// commitmentPok
		commitmentPok[0] = new(big.Int).SetBytes(proofBytes[FpSize*8+4+2*commitmentCount*FpSize : FpSize*8+4+2*commitmentCount*FpSize+FpSize])
		commitmentPok[1] = new(big.Int).SetBytes(proofBytes[FpSize*8+4+2*commitmentCount*FpSize+FpSize : FpSize*8+4+2*commitmentCount*FpSize+2*FpSize])

		proofStr += ","
		proofStr += fmt.Sprintf("[%s,%s]", commitments[0].String(), commitments[1].String())

		proofStr += ","
		proofStr += fmt.Sprintf("[%s,%s]", commitmentPok[0].String(), commitmentPok[1].String())
	}

	inputStr := g.GenSolInputParams()
	proofStr += "," + inputStr
	return proofStr
}

// 阅读gnark-solidity-checker的源码，简化下的input的处理流程
func (g *Groth16Wrapper) GenSolInputParams() string {
	if g.Curve != ecc.BN254 {
		logger.Fatal("only BN254 curve is supported")
	}
	bPublicWitness, err := g.WitnessPublic.MarshalBinary()
	if err != nil {
		logger.Fatal("marshal public witness failed: %v", err)
	}
	bPublicWitness = bPublicWitness[12:]
	nbInputs := len(bPublicWitness) / 32
	input := make([]*big.Int, nbInputs)
	for i := range input {
		var e fr_bn254.Element
		e.SetBytes(bPublicWitness[fr_bn254.Bytes*i : fr_bn254.Bytes*(i+1)])
		input[i] = new(big.Int)
		e.BigInt(input[i])
	}
	inputStr := "["
	for i := range input {
		inputStr += input[i].String() + ","
	}
	inputStr = inputStr[:len(inputStr)-1]
	inputStr += "]"
	return inputStr
}

// 编译和ABI生成
// groth16和plonk在这部分除solidity路径外，没有任何不同，但为了方便后续调用，分别在两个文件中添加了此函数
func (g *Groth16Wrapper) SolCompileAndABIgen(solPath string) {
	if solPath == "" {
		logger.Info("solPath is empty, use default path: output/Groth16Verifier.sol")
		solPath = "output/Groth16Verifier.sol"
	}
	compileCmd := exec.Command("solc", "--evm-version", "paris", "--combined-json", "abi,bin", solPath, "-o", "output", "--overwrite")
	logger.Debug("compile command: %v", compileCmd.String())
	if out, err := compileCmd.CombinedOutput(); err != nil {
		logger.Error("failed to compile: " + err.Error())
		logger.Error("output : " + string(out))
	} else {
		logger.Debug("compile success: " + string(out))
	}
	abiGenCmd := exec.Command("abigen", "--combined-json", filepath.Join("output", "combined.json"), "--pkg", "main", "--out", filepath.Join("output", "gnark_solidity.go"))
	logger.Debug("abiGen command: %v", abiGenCmd.String())
	if out, err := abiGenCmd.CombinedOutput(); err != nil {
		logger.Error("failed to generate abi: " + err.Error())
		logger.Error("output : " + string(out))
	} else {
		logger.Debug("abiGen success: " + string(out))
	}
}

func (g *Groth16Wrapper) GenNbCommitments() int {
	_proof, ok := g.Proof.(interface{ MarshalSolidity() []byte })
	if !ok {
		logger.Fatal("marshal proof failed")
	}
	proofBytes := _proof.MarshalSolidity()
	c := new(big.Int).SetBytes(proofBytes[FpSize*8 : FpSize*8+4])
	commitmentCount := int(c.Int64())
	return commitmentCount
}

func (g *Groth16Wrapper) SolGenMain() {
	helpers := template.FuncMap{
		"mul": func(a, b int) int {
			return a * b
		},
	}
	tmpl, err := template.New("").Funcs(helpers).Parse(Groth16Template)
	if err != nil {
		logger.Error("failed to parse template: " + err.Error())
	}
	prootStr := g.ProofMarshall()
	inputStr := g.PublicWitnessMarshall()
	nbCommitments := g.GenNbCommitments()
	logger.Info("nbCommitments: %d", nbCommitments)

	data := struct {
		Proof          string
		PublicInputs   string
		NbPublicInputs int
		NbCommitments  int
	}{
		Proof:          prootStr,
		PublicInputs:   inputStr,
		NbPublicInputs: len(g.WitnessPublic.Vector().(fr_bn254.Vector)),
		NbCommitments:  nbCommitments,
	}
	file, err := os.Create("output/main.go")
	if err != nil {
		logger.Error("failed to create file: " + err.Error())
	}
	defer file.Close()
	tmpl.Execute(file, data)
}

func (g *Groth16Wrapper) SolGenGoMod() {
	file, err := os.Create("output/go.mod")
	if err != nil {
		logger.Error("failed to create file: " + err.Error())
	}
	defer file.Close()
	tmpl, err := template.New("").Parse(GoModTemplate)
	if err != nil {
		logger.Error("failed to parse template: " + err.Error())
	}
	tmpl.Execute(file, nil)
}

func (g *Groth16Wrapper) SolVerify() {
	cmdGoModTidy := exec.Command("sh", "-c", "cd output && go mod tidy")
	logger.Info("running go mod tidy" + cmdGoModTidy.String())
	if out, err := cmdGoModTidy.CombinedOutput(); err != nil {
		logger.Error("failed to run go mod tidy: " + err.Error())
		logger.Error("output : " + string(out))
	} else {
		logger.Info("go mod tidy success: " + string(out))
	}

	cmdGoRun := exec.Command("sh", "-c", "cd output && go run main.go gnark_solidity.go")
	logger.Info("running go run main.go gnark_solidity.go" + cmdGoRun.String())
	if out, err := cmdGoRun.CombinedOutput(); err != nil {
		logger.Error("failed to run go run main.go gnark_solidity.go: " + err.Error())
		logger.Error("output : " + string(out))
	} else {
		logger.Info("go run main.go gnark_solidity.go success: " + string(out))
	}
}
