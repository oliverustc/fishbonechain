package gnarkwrapper

import (
	"gnarkabc/logger"
	"gnarkabc/utils"
	"testing"
)

func TestGroth16Write(t *testing.T) {
	utils.RemoveDir("output")
	for _, curveName := range CurveList {
		curve := CurveMap[curveName]
		var circuit TestCircuit
		circuit.PreCompile()
		zk := NewGroth16(&circuit, curve)
		zk.Compile()
		zk.Setup()
		circuit.Assign(curveName, 13, 17)
		zk.SetAssignment(&circuit)
		zk.Prove()
		zk.Verify()
		zk.WriteCCS("output/ccs_" + curveName)
		zk.WritePK("output/pk_" + curveName)
		zk.WriteVK("output/vk_" + curveName)
		zk.WriteWitness("output/witness_"+curveName, false)
		zk.WriteWitness("output/public_witness_"+curveName, true)
		zk.WriteProof("output/proof_" + curveName)
		logger.Info("write params success on [ %s ]", curveName)
	}
}

func TestGroth16Read(t *testing.T) {
	for _, curveName := range CurveList {
		curve := CurveMap[curveName]
		var circuit TestCircuit
		circuit.PreCompile()
		zk := NewGroth16(&circuit, curve)
		zk.ReadCCS("output/ccs_" + curveName)
		zk.ReadPK("output/pk_" + curveName)
		zk.ReadVK("output/vk_" + curveName)
		zk.ReadWitness("output/witness_"+curveName, false)
		// 首先基于已有参数自行prove和verify
		zk.Prove()
		zk.ReadWitness("output/public_witness_"+curveName, true)
		zk.Verify()
		// 然后读取已有的proof仅进行验证
		zk.ReadProof("output/proof_" + curveName)
		zk.Verify()
		logger.Info("prove and verify success on [ %s ] after read params", curveName)
	}
}
