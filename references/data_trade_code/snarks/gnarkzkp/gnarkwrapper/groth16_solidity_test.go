package gnarkwrapper

import (
	"gnarkabc/utils"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
)

func TestRemoveFileGroth16(t *testing.T) {
	utils.RemoveFile("output/Groth16Verifier.sol")
	utils.RemoveFile("output/combined.json")
	utils.RemoveFile("output/gnark_solidity.go")
	utils.RemoveFile("output/main.go")
	utils.RemoveDir("output")
}

func TestGroth16GenSolParams(t *testing.T) {
	var circuit TestCircuit
	circuit.PreCompile()
	zk := NewGroth16(&circuit, ecc.BN254)
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