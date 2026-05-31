package main

import (
	"gnarkabc/gnarkwrapper"
	"gnarkabc/hash/mimchash"
	"gnarkabc/logger"
	"gnarkabc/merkletree"
	"gnarkabc/utils"
	"strconv"
	"time"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

type RootObfuscationProof struct {
	Leaf  frontend.Variable `gnark:",public"`
	Path  []frontend.Variable
	Root0 frontend.Variable `gnark:",public"`
	Root1 frontend.Variable `gnark:",public"`
	Root2 frontend.Variable `gnark:",public"`
	Root3 frontend.Variable `gnark:",public"`

	Index0 frontend.Variable
	Index1 frontend.Variable
}

func (rop *RootObfuscationProof) Define(api frontend.API) error {
	h, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}
	var sum frontend.Variable
	node := rop.Leaf
	for len(rop.Path) != 0 {
		h.Reset()
		brotherHash := rop.Path[0]
		rop.Path = rop.Path[1:]
		h.Write(node)
		h.Write(brotherHash)
		sum = h.Sum()
		node = sum
	}
	api.Println("sum: ", sum)
	root := api.Lookup2(rop.Index0, rop.Index1, rop.Root0, rop.Root1, rop.Root2, rop.Root3)
	api.AssertIsEqual(sum, root)
	return nil
}

func (rop *RootObfuscationProof) PreCompile(args ...interface{}) {
	depth := args[0].(int)
	rop.Path = make([]frontend.Variable, depth)
}

func (rop *RootObfuscationProof) Assign(curveName string, args ...interface{}) {
	depth := args[0].(int)
	mod := gnarkwrapper.CurveMap[curveName].ScalarField()
	hashFunc := mimchash.MiMCCaseMap[curveName].Hash
	rop.Path = make([]frontend.Variable, depth)
	tree := merkletree.New(curveName, 2, depth)
	tree.RandConstruct()
	proofSet := tree.GenerateMerkleProof2(0)
	for i := range proofSet {
		rop.Path[i] = proofSet[i]
	}
	rop.Leaf = tree.LeavesList[0].Hash
	rop.Root0 = tree.Root.Hash
	rop.Root1 = mimchash.MiMCHash(hashFunc, [][]byte{mimchash.Convert2Byte(utils.RandStr(10), mod)})
	rop.Root2 = mimchash.MiMCHash(hashFunc, [][]byte{mimchash.Convert2Byte(utils.RandStr(10), mod)})
	rop.Root3 = mimchash.MiMCHash(hashFunc, [][]byte{mimchash.Convert2Byte(utils.RandStr(10), mod)})
	rop.Index0 = 0
	rop.Index1 = 0
}

func GenRootObfuscationZKP(scheme string, depthList []int) {
	logger.Info("generate " + scheme + " rootobfuscation proof css, pk, vk,witness...")
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]
	for _, depth := range depthList {
		var circuit RootObfuscationProof
		circuit.PreCompile(depth)
		zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
		zk.Compile()
		zk.Setup()
		zk.WriteCCS("output/" + scheme + "RootObfuscationProofCSS" + strconv.Itoa(depth))
		zk.WritePK("output/" + scheme + "RootObfuscationProofPK" + strconv.Itoa(depth))
		zk.WriteVK("output/" + scheme + "RootObfuscationProofVK" + strconv.Itoa(depth))
		circuit.Assign(curveName, depth)
		zk.SetAssignment(&circuit)
		zk.GenerateWitness(false)
		zk.WriteWitness("output/"+scheme+"RootObfuscationProofWitness"+strconv.Itoa(depth), false)
	}
	logger.Info("generate " + scheme + " rootobfuscation proof css, pk, vk, witness done")
}

func ProveRootObfuscationZKP(scheme string, depthList []int) {
	logger.Info("prove " + scheme + " rootobfuscation proof...")
	logger.Info("reading css, pk, vk, witness...")
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]
	var zkList []gnarkwrapper.GnarkWrapper
	var proveTime []time.Duration
	for _, depth := range depthList {
		var circuit RootObfuscationProof
		circuit.PreCompile(depth)
		zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
		zk.ReadCCS("output/" + scheme + "RootObfuscationProofCSS" + strconv.Itoa(depth))
		zk.ReadPK("output/" + scheme + "RootObfuscationProofPK" + strconv.Itoa(depth))
		zk.ReadVK("output/" + scheme + "RootObfuscationProofVK" + strconv.Itoa(depth))
		zk.ReadWitness("output/"+scheme+"RootObfuscationProofWitness"+strconv.Itoa(depth), false)
		zkList = append(zkList, zk)
	}
	logger.Info("read css, pk, vk, witness done, start to prove...")
	repeatNum := 20
	for _, zk := range zkList {
		start := time.Now()
		for range repeatNum {
			zk.Prove()
		}
		end := time.Now()
		proveTime = append(proveTime, end.Sub(start)/time.Duration(repeatNum))
	}
	logger.Info("prove " + scheme + " rootobfuscation proof done")
	for i, t := range proveTime {
		logger.Info("prove rootobfuscation proof with depth %d cost time: %v", depthList[i], t)
	}
	for i, zk := range zkList {
		zk.WriteProof("output/" + scheme + "RootObfuscationProofProof" + strconv.Itoa(depthList[i]))
		zk.GenerateWitness(true)
		zk.WriteWitness("output/"+scheme+"RootObfuscationProofPublicWitness"+strconv.Itoa(depthList[i]), true)
	}
	logger.Info("write " + scheme + " rootobfuscation proof and public witness done")
}

func VerifyRootObfuscationZKP(scheme string, depthList []int) {
	logger.Info("verify " + scheme + " rootobfuscation proof...")
	logger.Info("reading css, pk, vk, witness, proof...")
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]
	var zkList []gnarkwrapper.GnarkWrapper
	var verifyTime []time.Duration
	for _, depth := range depthList {
		var circuit RootObfuscationProof
		circuit.PreCompile(depth)
		zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
		zk.ReadCCS("output/" + scheme + "RootObfuscationProofCSS" + strconv.Itoa(depth))
		zk.ReadPK("output/" + scheme + "RootObfuscationProofPK" + strconv.Itoa(depth))
		zk.ReadVK("output/" + scheme + "RootObfuscationProofVK" + strconv.Itoa(depth))
		zk.ReadProof("output/" + scheme + "RootObfuscationProofProof" + strconv.Itoa(depth))
		zk.ReadWitness("output/"+scheme+"RootObfuscationProofPublicWitness"+strconv.Itoa(depth), true)
		zkList = append(zkList, zk)
	}
	logger.Info("read css, pk, vk, witness, proof done, start to verify...")
	repeatNum := 20
	for _, zk := range zkList {
		start := time.Now()
		for range repeatNum {
			zk.Verify()
		}
		end := time.Now()
		verifyTime = append(verifyTime, end.Sub(start)/time.Duration(repeatNum))
	}
	logger.Info("verify " + scheme + " rootobfuscation proof done")
	for i, t := range verifyTime {
		logger.Info("verify rootobfuscation proof with depth %d cost time: %v", depthList[i], t)
	}
}

func RootObfuscationProofExportSolidity(scheme string, depthList []int) {
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]
	var zkList []gnarkwrapper.GnarkWrapper
	for _, depth := range depthList {
		var circuit RootObfuscationProof
		circuit.PreCompile(depth)
		zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
		zk.ReadCCS("output/" + scheme + "RootObfuscationProofCSS" + strconv.Itoa(depth))
		zk.ReadPK("output/" + scheme + "RootObfuscationProofPK" + strconv.Itoa(depth))
		zk.ReadVK("output/" + scheme + "RootObfuscationProofVK" + strconv.Itoa(depth))
		zk.ReadProof("output/" + scheme + "RootObfuscationProofProof" + strconv.Itoa(depth))
		zk.ReadWitness("output/"+scheme+"RootObfuscationProofPublicWitness"+strconv.Itoa(depth), true)
		zkList = append(zkList, zk)
	}
	utils.EnsureDirExists("solidity")
	for i, zk := range zkList {
		zk.ExportSolidity("solidity/" + scheme + "RootObfuscationProofVerifier" + strconv.Itoa(depthList[i]) + ".sol")
		proofStr := zk.GenSolProofParams()
		logger.Info("proofStr:\n%s", proofStr)
	}
}
