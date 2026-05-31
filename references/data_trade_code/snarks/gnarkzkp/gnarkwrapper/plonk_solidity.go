package gnarkwrapper

import (
	"encoding/hex"
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

func (p *PlonkWrapper) ExportSolidity(filePath string) {
	if filePath == "" {
		logger.Info("filePath is empty, using default path: output/PlonkVerifier.sol")
		filePath = "output/PlonkVerifier.sol"
	}
	if p.Curve != ecc.BN254 {
		logger.Fatal("only BN254 curve is supported")
	}
	utils.EnsureDirExists("output")
	solFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		logger.Fatal("open file failed: %v", err)
	}
	defer solFile.Close()
	p.VK.ExportSolidity(solFile)
	logger.Info("export solidity to %s", filePath)
}

func (p *PlonkWrapper) ProofMarshall() (proofStr string) {
	if p.Curve != ecc.BN254 {
		logger.Fatal("only BN254 curve is supported")
	}

	_proof, ok := p.Proof.(interface{ MarshalSolidity() []byte })
	if !ok {
		logger.Fatal("marshal proof failed")
	}
	proofStr = hex.EncodeToString(_proof.MarshalSolidity())
	return
}

func (p *PlonkWrapper) PublicWitnessMarshall() (publicWitnessStr string) {
	if p.Curve != ecc.BN254 {
		logger.Fatal("only BN254 curve is supported")
	}
	bPublicWitness, err := p.WitnessPublic.MarshalBinary()
	if err != nil {
		logger.Fatal("marshal public witness failed: %v", err)
	}
	bPublicWitness = bPublicWitness[12:]
	publicWitnessStr = hex.EncodeToString(bPublicWitness)
	return
}

func (p *PlonkWrapper) GetPublicInputNum() string {
	if p.Curve != ecc.BN254 {
		logger.Fatal("only BN254 curve is supported")
	}
	return strconv.Itoa(len(p.WitnessPublic.Vector().(fr_bn254.Vector)))
}

func (p *PlonkWrapper) GenSolProofParams() (proofStr string) {
	if p.Curve != ecc.BN254 {
		logger.Fatal("only BN254 curve is supported")
	}

	_proof, ok := p.Proof.(interface{ MarshalSolidity() []byte })
	if !ok {
		logger.Fatal("marshal proof failed")
	}
	proofStr = hex.EncodeToString(_proof.MarshalSolidity())
	proofStr = "0x" + proofStr
	inputStr := p.GenSolInputParams()
	proofStr += "," + inputStr
	return
}

func (p *PlonkWrapper) GenSolInputParams() (inputStr string) {
	if p.Curve != ecc.BN254 {
		logger.Fatal("only BN254 curve is supported")
	}
	bPublicWitness, err := p.WitnessPublic.MarshalBinary()
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
	inputStr = "["
	for i := range input {
		inputStr += input[i].String() + ","
	}
	inputStr = inputStr[:len(inputStr)-1]
	inputStr += "]"
	return
}

// 编译和ABI生成
// groth16和plonk在这部分除solidity路径外，没有任何不同，但为了方便后续调用，分别在两个文件中添加了此函数
func (p *PlonkWrapper) SolCompileAndABIgen(solPath string) {
	if solPath == "" {
		logger.Info("solPath is empty, use default path: output/PlonkVerifier.sol")
		solPath = "output/PlonkVerifier.sol"
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

func (p *PlonkWrapper) SolGenMain() {
	helpers := template.FuncMap{
		"mul": func(a, b int) int {
			return a * b
		},
	}
	tmpl, err := template.New("").Funcs(helpers).Parse(PlonkTemplate)
	if err != nil {
		logger.Error("failed to parse template: " + err.Error())
	}
	prootStr := p.ProofMarshall()
	inputStr := p.PublicWitnessMarshall()

	data := struct {
		Proof          string
		PublicInputs   string
		NbPublicInputs int
		NbCommitments  int
	}{
		Proof:          prootStr,
		PublicInputs:   inputStr,
		NbPublicInputs: len(p.WitnessPublic.Vector().(fr_bn254.Vector)),
		// 暂时不懂怎么获取commitments的数量，所以先设置为0
		NbCommitments: 0,
	}
	file, err := os.Create("output/main.go")
	if err != nil {
		logger.Error("failed to create file: " + err.Error())
	}
	defer file.Close()
	tmpl.Execute(file, data)
}

func (p *PlonkWrapper) SolGenGoMod() {
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

func (p *PlonkWrapper) SolVerify() {
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
