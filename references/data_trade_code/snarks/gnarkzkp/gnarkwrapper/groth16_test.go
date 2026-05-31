package gnarkwrapper

import (
	"gnarkabc/logger"
	"gnarkabc/utils"
	"testing"
)

func TestGroth16(t *testing.T) {
	var circuit TestCircuit
	circuit.PreCompile()
	for _, curveName := range CurveList {
		curve := CurveMap[curveName]
		zk := NewGroth16(&circuit, curve)
		zk.Compile()
		zk.Setup()
		compileTime := zk.BenchmarkCompile(10)
		setupTime := zk.BenchmarkSetup(10)

		p := utils.RandInt(0, 1000)
		q := utils.RandInt(0, 1000)
		circuit.Assign(curveName, p, q)
		zk.SetAssignment(&circuit)
		zk.Prove()
		zk.Verify()
		logger.Info("groth16 on curve [ %s ] success", curveName)

		proveTime := zk.BenchmarkProve(10)
		verifyTime := zk.BenchmarkVerify(10)

		logger.Info("benchmark on compile : %s", compileTime.String())
		logger.Info("benchmark on setup : %s", setupTime.String())
		logger.Info("benchmark on prove : %s", proveTime.String())
		logger.Info("benchmark on verify : %s", verifyTime.String())
	}
}
