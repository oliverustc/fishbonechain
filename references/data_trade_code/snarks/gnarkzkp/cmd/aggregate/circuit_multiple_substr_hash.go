package main

import (
	"gnarkabc/gnarkwrapper"
	"gnarkabc/hash/mimchash"
	"gnarkabc/logger"
	"gnarkabc/utils"
	"strconv"
	"time"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

type SubstrHashProof struct {
	PreImage       [30]frontend.Variable
	Hash           [10]frontend.Variable `gnark:",public"`
	PublicPreImage [10]frontend.Variable `gnark:",public"`
}

func checkSubstrHashProof(api frontend.API, mimchash mimc.MiMC, preImage [3]frontend.Variable, hash frontend.Variable, publicPreImage frontend.Variable) {
	mimchash.Reset()
	for _, preImage := range preImage {
		mimchash.Write(preImage)
	}
	api.AssertIsEqual(mimchash.Sum(), hash)
	api.AssertIsEqual(publicPreImage, preImage[0])
}

func (c *SubstrHashProof) Define(api frontend.API) error {
	mimc, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}
	mimc.Reset()
	for i := 0; i < 10; i++ {
		checkSubstrHashProof(api, mimc, [3]frontend.Variable{c.PreImage[i*3], c.PreImage[i*3+1], c.PreImage[i*3+2]}, c.Hash[i], c.PublicPreImage[i])
	}
	return nil
}

func (c *SubstrHashProof) PreCompile() {
}

func (c *SubstrHashProof) Assign(curveName string, params ...interface{}) {
	mod := gnarkwrapper.CurveMap[curveName].ScalarField()
	hashFunc := mimchash.MiMCCaseMap[curveName].Hash
	for i := 0; i < 10; i++ {
		preImage0Str := utils.RandStr(10)
		preImage1Str := utils.RandStr(10)
		preImage2Str := utils.RandStr(10)

		preImage0 := mimchash.Convert2Byte(preImage0Str, mod)
		preImage1 := mimchash.Convert2Byte(preImage1Str, mod)
		preImage2 := mimchash.Convert2Byte(preImage2Str, mod)

		hash := mimchash.MiMCHash(hashFunc, [][]byte{preImage0, preImage1, preImage2})

		c.PreImage[i*3] = preImage0
		c.PreImage[i*3+1] = preImage1
		c.PreImage[i*3+2] = preImage2
		c.Hash[i] = hash
		c.PublicPreImage[i] = preImage0
	}
}

func GenMultipleSubstrHashZKP(scheme string, num int) {
	logger.Info("generate " + scheme + " multiple substrhash proof css, pk, vk,witness...")
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]
	var circuit SubstrHashProof
	circuit.PreCompile()
	zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
	zk.Compile()
	zk.Setup()
	zk.WriteCCS("output/" + scheme + "MultipleSubstrHashProofCSS")
	zk.WritePK("output/" + scheme + "MultipleSubstrHashProofPK")
	zk.WriteVK("output/" + scheme + "MultipleSubstrHashProofVK")
	for i := range num {
		circuit.Assign(curveName)
		zk.SetAssignment(&circuit)
		zk.GenerateWitness(false)
		zk.WriteWitness("output/"+scheme+"MultipleSubstrHashProofWitness"+strconv.Itoa(i), false)
	}
	logger.Info("generate " + scheme + " multiple substrhash proof css, pk, vk, witness done")
}

func ProveMultipleSubstrHashZKP(scheme string, num int) {
	logger.Info("prove " + scheme + " multiple substrhash proof ...")
	logger.Info("reading css, pk, vk, witness...")
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]
	var circuit SubstrHashProof
	circuit.PreCompile()
	var zkList []gnarkwrapper.GnarkWrapper
	for i := range num {
		zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
		zk.ReadCCS("output/" + scheme + "MultipleSubstrHashProofCSS")
		zk.ReadPK("output/" + scheme + "MultipleSubstrHashProofPK")
		zk.ReadVK("output/" + scheme + "MultipleSubstrHashProofVK")
		zk.ReadWitness("output/"+scheme+"MultipleSubstrHashProofWitness"+strconv.Itoa(i), false)
		zkList = append(zkList, zk)
	}
	logger.Info("read css, pk, vk, witness done, start to prove...")
	start := time.Now()
	for _, zk := range zkList {
		zk.Prove()
	}
	end := time.Now()
	logger.Info("prove " + scheme + " multiple substrhash proof done")
	logger.Info("prove %d multiple substrhash proof cost time: %v", num, end.Sub(start))
	logger.Info("every prove cost time: %v", end.Sub(start)/time.Duration(num))
	for i, zk := range zkList {
		zk.WriteProof("output/" + scheme + "MultipleSubstrHashProofProof" + strconv.Itoa(i))
		zk.GenerateWitness(true)
		zk.WriteWitness("output/"+scheme+"MultipleSubstrHashProofPublicWitness"+strconv.Itoa(i), true)
	}
	logger.Info("write " + scheme + " multiple substrhash proof and public witness done")
}

func VerifyMultipleSubstrHashZKP(scheme string, num int) {
	logger.Info("verify " + scheme + " multiple substrhash proof ...")
	logger.Info("reading css, pk, vk, witness, proof...")
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]
	var circuit SubstrHashProof
	circuit.PreCompile()
	var zkList []gnarkwrapper.GnarkWrapper
	for i := range num {
		zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
		zk.ReadCCS("output/" + scheme + "MultipleSubstrHashProofCSS")
		zk.ReadPK("output/" + scheme + "MultipleSubstrHashProofPK")
		zk.ReadVK("output/" + scheme + "MultipleSubstrHashProofVK")
		zk.ReadWitness("output/"+scheme+"MultipleSubstrHashProofPublicWitness"+strconv.Itoa(i), true)
		zk.ReadProof("output/" + scheme + "MultipleSubstrHashProofProof" + strconv.Itoa(i))
		zkList = append(zkList, zk)
	}
	logger.Info("read css, pk, vk, witness, proof done, start to verify...")
	start := time.Now()
	for _, zk := range zkList {
		zk.Verify()
	}
	end := time.Now()
	logger.Info("verify " + scheme + " multiple substrhash proof done")
	logger.Info("verify %d multiple substrhash proof cost time: %v", num, end.Sub(start))
	logger.Info("every verify cost time: %v", end.Sub(start)/time.Duration(num))
}

func SubstrHashProofExportSolidity(scheme string) {
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]
	var circuit SubstrHashProof
	circuit.PreCompile()
	zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
	zk.ReadCCS("output/" + scheme + "SubstrHashProofCSS")
	zk.ReadPK("output/" + scheme + "SubstrHashProofPK")
	zk.ReadVK("output/" + scheme + "SubstrHashProofVK")
	zk.ReadProof("output/" + scheme + "SubstrHashProofProof0")
	zk.ReadWitness("output/"+scheme+"SubstrHashProofPublicWitness0", true)
	utils.EnsureDirExists("solidity")
	zk.ExportSolidity("solidity/" + scheme + "SubstrHashProofVerifier.sol")
	proofStr := zk.GenSolProofParams()
	logger.Info("proofStr:\n%s", proofStr)
}
