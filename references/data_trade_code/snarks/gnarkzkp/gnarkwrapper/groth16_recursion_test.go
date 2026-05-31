package gnarkwrapper

import (
	"gnarkabc/logger"
	"gnarkabc/utils"
	"strconv"
	"testing"

	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bn254"
)

func GetGroth16Inner(curveName string) (constraint.ConstraintSystem, groth16.VerifyingKey, witness.Witness, groth16.Proof) {
	curve := CurveMap[curveName]
	var circuit TestCircuit
	circuit.PreCompile()
	g := NewGroth16(&circuit, curve)
	g.Compile()
	g.Setup()
	p := utils.RandInt(1, 200)
	q := utils.RandInt(1, 200)
	circuit.Assign(curveName, p, q)
	g.SetAssignment(&circuit)
	g.Prove()
	g.Verify()
	return g.CCS, g.VK, g.WitnessFull, g.Proof
}

func TestOneRecursion(t *testing.T) {
	curveName := "BN254"
	curve := CurveMap[curveName]
	var circuit TestCircuit
	circuit.PreCompile()
	g := NewGroth16(&circuit, curve)
	g.Compile()
	g.Setup()
	p := utils.RandInt(1, 200)
	q := utils.RandInt(1, 200)
	circuit.Assign(curveName, p, q)
	g.SetAssignment(&circuit)
	g.Prove()
	g.Verify()

	logger.Info("Try to recursion")
	var outerCircuit OuterCircuit[sw_bn254.ScalarField, sw_bn254.G1Affine, sw_bn254.G2Affine, sw_bn254.GTEl]
	outerCircuit.PreCompile(g)
	outerGroth16 := NewGroth16(&outerCircuit, curve)
	outerGroth16.Compile()
	outerGroth16.Setup()
	outerCircuit.Assign(curveName, g)
	outerGroth16.SetAssignment(&outerCircuit)
	outerGroth16.Prove()
	outerGroth16.Verify()
}

func TestOneRecursionConstant(t *testing.T) {
	curveName := "BN254"
	curve := CurveMap[curveName]
	var circuit TestCircuit
	circuit.PreCompile()
	g := NewGroth16(&circuit, curve)
	g.Compile()
	g.Setup()
	p := utils.RandInt(1, 200)
	q := utils.RandInt(1, 200)
	circuit.Assign(curveName, p, q)
	g.SetAssignment(&circuit)
	g.Prove()
	g.Verify()

	logger.Info("Try to recursion")
	var outerCircuit OuterCircuitConstant[sw_bn254.ScalarField, sw_bn254.G1Affine, sw_bn254.G2Affine, sw_bn254.GTEl]
	outerCircuit.PreCompile(g)
	outerGroth16 := NewGroth16(&outerCircuit, curve)
	outerGroth16.Compile()
	outerGroth16.Setup()
	outerCircuit.Assign(curveName, g)
	outerGroth16.SetAssignment(&outerCircuit)
	outerGroth16.Prove()
	outerGroth16.Verify()
}

func GenerateMultipleProof(innerNum int){
	curveName := "BN254"
	curve := CurveMap[curveName]
	var tcircuit TestCircuit
	tcircuit.PreCompile()
	tg := NewGroth16(&tcircuit, curve)
	tg.Compile()
	tg.Setup()
	tg.WriteCCS("output/innerCSS")
	for i := range innerNum {
		p := utils.RandInt(1, 200)
		q := utils.RandInt(1, 200)
		tcircuit.Assign(curveName, p, q)
		tg.SetAssignment(&tcircuit)
		tg.Prove()
		tg.Verify()
		tg.WriteProof("output/innerProof" + strconv.Itoa(i))
		tg.WriteWitness("output/innerWitness"+strconv.Itoa(i), false)
	}
}

func TestRecursionAggregate(t *testing.T) {
	innerNum := 10
	curveName := "BN254"
	curve := CurveMap[curveName]
	var circuit TestCircuit
	circuit.PreCompile()
	g := NewGroth16(&circuit, curve)
	g.ReadCCS("output/innerCSS")
	var aggregateCircuit AggregateCircuit[sw_bn254.ScalarField, sw_bn254.G1Affine, sw_bn254.G2Affine, sw_bn254.GTEl]
	aggregateCircuit.PreCompile(g.CCS, innerNum)
	aggregateGroth16 := NewGroth16(&aggregateCircuit, curve)
	aggregateGroth16.Compile()
	aggregateGroth16.Setup()
	aggregateGroth16.WriteCCS("output/aggregateCSS")
	aggregateGroth16.WritePK("output/aggregatePK")
	aggregateGroth16.WriteVK("output/aggregateVK")
	var proofList []groth16.Proof
	var witnessList []witness.Witness
	for i := range innerNum {
		g.ReadProof("output/innerProof" + strconv.Itoa(i))
		g.ReadWitness("output/innerWitness"+strconv.Itoa(i), false)
		proofList = append(proofList, g.Proof)
		witnessList = append(witnessList, g.WitnessFull)
	}
	aggregateCircuit.Assign("BN254", proofList, g.VK, witnessList)
	aggregateGroth16.SetAssignment(&aggregateCircuit)
	aggregateGroth16.Prove()
	aggregateGroth16.Verify()
}
