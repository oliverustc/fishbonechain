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

type SubsetHashProof struct {
	PreImage frontend.Variable
	Nonce    frontend.Variable

	Item0  frontend.Variable `gnark:",public"`
	Item1  frontend.Variable `gnark:",public"`
	Item2  frontend.Variable `gnark:",public"`
	Item3  frontend.Variable `gnark:",public"`
	Index0 frontend.Variable
	Index1 frontend.Variable
}

func (c *SubsetHashProof) Define(api frontend.API) error {
	mimc, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}
	mimc.Reset()
	mimc.Write(c.PreImage)
	mimc.Write(c.Nonce)
	hash := mimc.Sum()
	item := api.Lookup2(c.Index0, c.Index1, c.Item0, c.Item1, c.Item2, c.Item3)
	api.AssertIsEqual(item, hash)
	return nil
}

func (c *SubsetHashProof) PreCompile() {
}

func (c *SubsetHashProof) Assign(curveName string, params ...interface{}) {
	mod := gnarkwrapper.CurveMap[curveName].ScalarField()
	item0 := utils.RandStr(10)
	item1 := utils.RandStr(10)
	item2 := utils.RandStr(10)
	item3 := utils.RandStr(10)
	nonce := utils.RandStr(10)

	nonceBytes := mimchash.Convert2Byte(nonce, mod)

	item0Bytes := mimchash.Convert2Byte(item0, mod)
	item1Bytes := mimchash.Convert2Byte(item1, mod)
	item2Bytes := mimchash.Convert2Byte(item2, mod)
	item3Bytes := mimchash.Convert2Byte(item3, mod)

	hashFunc := mimchash.MiMCCaseMap[curveName].Hash
	item0Hash := mimchash.MiMCHash(hashFunc, [][]byte{item0Bytes, nonceBytes})
	item1Hash := mimchash.MiMCHash(hashFunc, [][]byte{item1Bytes, nonceBytes})
	item2Hash := mimchash.MiMCHash(hashFunc, [][]byte{item2Bytes, nonceBytes})
	item3Hash := mimchash.MiMCHash(hashFunc, [][]byte{item3Bytes, nonceBytes})

	c.Item0 = item0Hash
	c.Item1 = item1Hash
	c.Item2 = item2Hash
	c.Item3 = item3Hash

	c.PreImage = item2Bytes
	c.Nonce = nonceBytes

	c.Index0 = new(big.Int).SetInt64(0)
	c.Index1 = new(big.Int).SetInt64(1)
}

func GenSubsetHashZKP(scheme string, num int) {
	logger.Info("generate " + scheme + " subsethash proof css, pk, vk,witness...")
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]
	var circuit SubsetHashProof
	circuit.PreCompile()
	zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
	zk.Compile()
	zk.Setup()
	zk.WriteCCS("output/" + scheme + "SubsetHashProofCSS")
	zk.WritePK("output/" + scheme + "SubsetHashProofPK")
	zk.WriteVK("output/" + scheme + "SubsetHashProofVK")
	for i := range num {
		circuit.Assign(curveName)
		zk.SetAssignment(&circuit)
		zk.GenerateWitness(false)
		zk.WriteWitness("output/"+scheme+"SubsetHashProofWitness"+strconv.Itoa(i), false)
	}
	logger.Info("generate " + scheme + " subsethash proof css, pk, vk, witness done")
}

func ProveSubsetHashZKP(scheme string, num int) {
	logger.Info("prove " + scheme + " subsethash proof ...")
	logger.Info("reading css, pk, vk, witness...")
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]
	var circuit SubsetHashProof
	circuit.PreCompile()
	var zkList []gnarkwrapper.GnarkWrapper
	for i := range num {
		zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
		zk.ReadCCS("output/" + scheme + "SubsetHashProofCSS")
		zk.ReadPK("output/" + scheme + "SubsetHashProofPK")
		zk.ReadVK("output/" + scheme + "SubsetHashProofVK")
		zk.ReadWitness("output/"+scheme+"SubsetHashProofWitness"+strconv.Itoa(i), false)
		zkList = append(zkList, zk)
	}
	logger.Info("read css, pk, vk, witness done, start to prove...")
	start := time.Now()
	for _, zk := range zkList {
		zk.Prove()
	}
	end := time.Now()
	logger.Info("prove " + scheme + " subsethash proof done")
	logger.Info("prove %d subsethash proof cost time: %v", num, end.Sub(start))
	logger.Info("every prove cost time: %v", end.Sub(start)/time.Duration(num))
	for i, zk := range zkList {
		zk.WriteProof("output/" + scheme + "SubsetHashProofProof" + strconv.Itoa(i))
		zk.GenerateWitness(true)
		zk.WriteWitness("output/"+scheme+"SubsetHashProofPublicWitness"+strconv.Itoa(i), true)
	}
	logger.Info("write " + scheme + " subsethash proof and public witness done")
}

func VerifySubsetHashZKP(scheme string, num int) {
	logger.Info("verify " + scheme + " subsethash proof ...")
	logger.Info("reading css, pk, vk, witness, proof...")
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]
	var circuit SubsetHashProof
	circuit.PreCompile()
	var zkList []gnarkwrapper.GnarkWrapper
	for i := range num {
		zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
		zk.ReadCCS("output/" + scheme + "SubsetHashProofCSS")
		zk.ReadPK("output/" + scheme + "SubsetHashProofPK")
		zk.ReadVK("output/" + scheme + "SubsetHashProofVK")
		zk.ReadWitness("output/"+scheme+"SubsetHashProofWitness"+strconv.Itoa(i), false)
		zk.ReadProof("output/" + scheme + "SubsetHashProofProof" + strconv.Itoa(i))
		zk.ReadWitness("output/"+scheme+"SubsetHashProofPublicWitness"+strconv.Itoa(i), true)
		zkList = append(zkList, zk)
	}
	logger.Info("read css, pk, vk, witness, proof done, start to verify...")
	start := time.Now()
	for _, zk := range zkList {
		zk.Verify()
	}
	end := time.Now()
	logger.Info("verify " + scheme + " subsethash proof done")
	logger.Info("verify %d subsethash proof cost time: %v", num, end.Sub(start))
	logger.Info("every verify cost time: %v", end.Sub(start)/time.Duration(num))
}

func SubsetHashProofExportSolidity(scheme string) {
	curveName := "BN254"
	curve := gnarkwrapper.CurveMap[curveName]

	var circuit SubsetHashProof
	circuit.PreCompile()
	zk := gnarkwrapper.NewGnarkWrapper(scheme, &circuit, curve)
	zk.ReadCCS("output/" + scheme + "SubsetHashProofCSS")
	zk.ReadPK("output/" + scheme + "SubsetHashProofPK")
	zk.ReadVK("output/" + scheme + "SubsetHashProofVK")
	zk.ReadProof("output/" + scheme + "SubsetHashProofProof0")
	zk.ReadWitness("output/"+scheme+"SubsetHashProofPublicWitness0", true)
	utils.EnsureDirExists("solidity")
	zk.ExportSolidity("solidity/" + scheme + "SubsetHashProofVerifier.sol")
	proofStr := zk.GenSolProofParams()
	logger.Info("proofStr:\n%s", proofStr)
}
