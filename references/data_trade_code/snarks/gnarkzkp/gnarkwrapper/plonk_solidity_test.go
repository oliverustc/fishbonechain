package gnarkwrapper

import (
	"gnarkabc/utils"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
)

func TestRemoveFilePlonk(t *testing.T) {
	utils.RemoveFile("output/PlonkVerifier.sol")
	utils.RemoveFile("output/combined.json")
	utils.RemoveFile("output/gnark_solidity.go")
	utils.RemoveFile("output/main.go")
	utils.RemoveDir("output")
}

func TestPlonkGenSolParams(t *testing.T) {
	var circuit TestCircuit
	circuit.PreCompile()
	zk := NewPlonk(&circuit, ecc.BN254)
	zk.Compile()
	zk.Setup()
	circuit.Assign("BN254", 13, 17)
	zk.SetAssignment(&circuit)
	zk.Prove()
	zk.Verify()
	zk.ExportSolidity("")
	prootStr := zk.GenSolProofParams()
	t.Logf("proofStr:\n%s", prootStr)
	inputStr := zk.GenSolInputParams()
	t.Logf("inputStr:\n%s", inputStr)
	zk.SolCompileAndABIgen("")
	zk.SolGenMain()
	zk.SolGenGoMod()
	zk.SolVerify()
}
