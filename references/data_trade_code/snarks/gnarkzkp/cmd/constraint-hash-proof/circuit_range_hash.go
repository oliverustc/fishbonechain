package main

import (
	"gnarkabc/gnarkwrapper"
	"gnarkabc/hash/mimchash"
	"gnarkabc/logger"
	"gnarkabc/utils"
	"math/big"
	"strconv"
	"time"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
)

type RangeHashProof struct {
	PreImage frontend.Variable
	Hash     frontend.Variable `gnark:",public"`
	Min      frontend.Variable `gnark:",public"`
	Max      frontend.Variable `gnark:",public"`
}

func (c *RangeHashProof) Define(api frontend.API) error {
	mimc, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}
	mimc.Reset()
	mimc.Write(c.PreImage)
	hash := mimc.Sum()
	api.AssertIsEqual(hash, c.Hash)
	api.AssertIsLessOrEqual(c.Min, c.PreImage)
	api.AssertIsLessOrEqual(c.PreImage, c.Max)
	return nil
}

func (c *RangeHashProof) PreCompile(params ...interface{}) {

}

func (c *RangeHashProof) Assign(curveName string, params ...interface{}) {
	mod := gnarkwrapper.CurveMap[curveName].ScalarField()
	preImageStr := utils.RandStr(3)
	preImage := mimchash.Convert2Byte(preImageStr, mod)
	c.PreImage = preImage
	c.Min = 0
	c.Max = new(big.Int).Sub(mod, new(big.Int).SetInt64(1))
	hashFunc := mimchash.MiMCCaseMap[curveName].Hash
	hash := mimchash.MiMCHash(hashFunc, [][]byte{preImage})
	c.Hash = hash
}

func GenRangeHashZKP(scheme string, num int) {
	logger.Info("generate " + scheme + " rangehash proof css, pk, vk,witness...")
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]
	var circuit RangeHashProof
	circuit.PreCompile()
	zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
	zk.Compile()
	zk.Setup()
	zk.WriteCCS("output/" + scheme + "RangeHashProofCSS")
	zk.WritePK("output/" + scheme + "RangeHashProofPK")
	zk.WriteVK("output/" + scheme + "RangeHashProofVK")
	for i := range num {
		circuit.Assign(curveName)
		zk.SetAssignment(&circuit)
		zk.GenerateWitness(false)
		zk.WriteWitness("output/"+scheme+"RangeHashProofWitness"+strconv.Itoa(i), false)
	}
	logger.Info("generate " + scheme + " rangehash css, pk, vk, witness done")
}

func ProveRangeHashZKP(scheme string, num int) {
	logger.Info("prove " + scheme + " rangehash proof ...")
	logger.Info("reading css, pk, vk, witness...")
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]
	var circuit RangeHashProof
	circuit.PreCompile()
	var zkList []gnarkwrapper.GnarkWrapper
	for i := range num {
		zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
		zk.ReadCCS("output/" + scheme + "RangeHashProofCSS")
		zk.ReadPK("output/" + scheme + "RangeHashProofPK")
		zk.ReadVK("output/" + scheme + "RangeHashProofVK")
		zk.ReadWitness("output/"+scheme+"RangeHashProofWitness"+strconv.Itoa(i), false)
		zkList = append(zkList, zk)
	}
	logger.Info("read css, pk, vk, witness done, start to prove...")
	start := time.Now()
	for _, zk := range zkList {
		zk.Prove()
	}
	end := time.Now()
	logger.Info("prove " + scheme + " rangehash proof done")
	logger.Info("prove %d rangehash proof cost time: %v", num, end.Sub(start))
	logger.Info("every prove cost time: %v", end.Sub(start)/time.Duration(num))
	for i, zk := range zkList {
		zk.WriteProof("output/" + scheme + "RangeHashProofProof" + strconv.Itoa(i))
		zk.GenerateWitness(true)
		zk.WriteWitness("output/"+scheme+"RangeHashProofPublicWitness"+strconv.Itoa(i), true)
	}
	logger.Info("write " + scheme + " rangehash proof and public witness done")
}

func VerifyRangeHashZKP(scheme string, num int) {
	logger.Info("verify " + scheme + " rangehash proof ...")
	logger.Info("reading css, pk, vk, witness, proof...")
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]
	var circuit RangeHashProof
	circuit.PreCompile()
	var zkList []gnarkwrapper.GnarkWrapper
	for i := range num {
		zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
		zk.ReadCCS("output/" + scheme + "RangeHashProofCSS")
		zk.ReadPK("output/" + scheme + "RangeHashProofPK")
		zk.ReadVK("output/" + scheme + "RangeHashProofVK")
		zk.ReadWitness("output/"+scheme+"RangeHashProofWitness"+strconv.Itoa(i), false)
		zk.ReadProof("output/" + scheme + "RangeHashProofProof" + strconv.Itoa(i))
		zk.ReadWitness("output/"+scheme+"RangeHashProofPublicWitness"+strconv.Itoa(i), true)
		zkList = append(zkList, zk)
	}
	logger.Info("read css, pk, vk, witness, proof done, start to verify...")
	start := time.Now()
	for _, zk := range zkList {
		zk.Verify()
	}
	end := time.Now()
	logger.Info("verify " + scheme + " rangehash proof done")
	logger.Info("verify %d rangehash proof cost time: %v", num, end.Sub(start))
	logger.Info("every verify cost time: %v", end.Sub(start)/time.Duration(num))
}

func RangeHashProofExportSolidity(scheme string) {
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]

	var circuit RangeHashProof
	circuit.PreCompile()
	zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
	zk.ReadCCS("output/" + scheme + "RangeHashProofCSS")
	zk.ReadPK("output/" + scheme + "RangeHashProofPK")
	zk.ReadVK("output/" + scheme + "RangeHashProofVK")
	zk.ReadProof("output/" + scheme + "RangeHashProofProof0")
	zk.ReadWitness("output/"+scheme+"RangeHashProofPublicWitness0", true)
	utils.EnsureDirExists("solidity")
	zk.ExportSolidity("solidity/" + scheme + "RangeHashProofVerifier.sol")
	proofStr := zk.GenSolProofParams()
	logger.Info("proofStr:\n%s", proofStr)
}
