package gnarkwrapper

import (
	"fmt"
	"gnarkabc/logger"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra"
	"github.com/consensys/gnark/std/math/emulated"
	recursion_groth16 "github.com/consensys/gnark/std/recursion/groth16"
)

type OuterCircuit[FR emulated.FieldParams, G1El algebra.G1ElementT, G2El algebra.G2ElementT, GtEl algebra.GtElementT] struct {
	Proof        recursion_groth16.Proof[G1El, G2El]
	VerifyingKey recursion_groth16.VerifyingKey[G1El, G2El, GtEl]
	InnerWitness recursion_groth16.Witness[FR]
}

func (c *OuterCircuit[FR, G1El, G2El, GtEl]) Define(api frontend.API) error {
	verifier, err := recursion_groth16.NewVerifier[FR, G1El, G2El, GtEl](api)
	if err != nil {
		return fmt.Errorf("new verifier: %w", err)
	}
	return verifier.AssertProof(c.VerifyingKey, c.Proof, c.InnerWitness)
}

func (c *OuterCircuit[FR, G1El, G2El, GtEl]) PreCompile(args ...interface{}) {
	innerGroth16 := args[0].(*Groth16Wrapper)
	c.Proof = recursion_groth16.PlaceholderProof[G1El, G2El](innerGroth16.CCS)
	c.InnerWitness = recursion_groth16.PlaceholderWitness[FR](innerGroth16.CCS)
	c.VerifyingKey = recursion_groth16.PlaceholderVerifyingKey[G1El, G2El, GtEl](innerGroth16.CCS)
}

func (c *OuterCircuit[FR, G1El, G2El, GtEl]) Assign(curveName string, args ...interface{}) {
	innerGroth16 := args[0].(*Groth16Wrapper)
	circuitVK, err := recursion_groth16.ValueOfVerifyingKey[G1El, G2El, GtEl](innerGroth16.VK)
	if err != nil {
		logger.Fatal("initializing verifying key failed, error: %s", err)
	}
	circuitWitness, err := recursion_groth16.ValueOfWitness[FR](innerGroth16.WitnessFull)
	if err != nil {
		logger.Fatal("initializing witness failed, error: %s", err)
	}
	circuitProof, err := recursion_groth16.ValueOfProof[G1El, G2El](innerGroth16.Proof)
	if err != nil {
		logger.Fatal("initializing proof failed, error: %s", err)
	}
	c.VerifyingKey = circuitVK
	c.InnerWitness = circuitWitness
	c.Proof = circuitProof
}

type OuterCircuitConstant[FR emulated.FieldParams, G1El algebra.G1ElementT, G2El algebra.G2ElementT, GtEl algebra.GtElementT] struct {
	Proof        recursion_groth16.Proof[G1El, G2El]
	vk           recursion_groth16.VerifyingKey[G1El, G2El, GtEl] `gnark:"-"`
	InnerWitness recursion_groth16.Witness[FR]
}

func (c *OuterCircuitConstant[FR, G1El, G2El, GtEl]) Define(api frontend.API) error {
	verifier, err := recursion_groth16.NewVerifier[FR, G1El, G2El, GtEl](api)
	if err != nil {
		return fmt.Errorf("new verifier: %w", err)
	}
	return verifier.AssertProof(c.vk, c.Proof, c.InnerWitness)
}

func (c *OuterCircuitConstant[FR, G1El, G2El, GtEl]) PreCompile(args ...interface{}) {
	innerGroth16 := args[0].(*Groth16Wrapper)
	circuitVK, err := recursion_groth16.ValueOfVerifyingKey[G1El, G2El, GtEl](innerGroth16.VK)
	if err != nil {
		logger.Fatal("initializing verifying key failed, error: %s", err)
	}
	c.InnerWitness = recursion_groth16.PlaceholderWitness[FR](innerGroth16.CCS)
	c.vk = circuitVK
}

func (c *OuterCircuitConstant[FR, G1El, G2El, GtEl]) Assign(curveName string, args ...interface{}) {
	innerGroth16 := args[0].(*Groth16Wrapper)
	circuitWitness, err := recursion_groth16.ValueOfWitness[FR](innerGroth16.WitnessFull)
	if err != nil {
		logger.Fatal("initializing witness failed, error: %s", err)
	}
	circuitProof, err := recursion_groth16.ValueOfProof[G1El, G2El](innerGroth16.Proof)
	if err != nil {
		logger.Fatal("initializing proof failed, error: %s", err)
	}
	c.InnerWitness = circuitWitness
	c.Proof = circuitProof
}

type AggregateCircuit[FR emulated.FieldParams, G1El algebra.G1ElementT, G2El algebra.G2ElementT, GtEl algebra.GtElementT] struct {
	Proofs        []recursion_groth16.Proof[G1El, G2El]
	VerifyingKey  recursion_groth16.VerifyingKey[G1El, G2El, GtEl]
	InnerWitnesss []recursion_groth16.Witness[FR]
}

func (a *AggregateCircuit[FR, G1El, G2El, GtEl]) Define(api frontend.API) error {
	verifier, err := recursion_groth16.NewVerifier[FR, G1El, G2El, GtEl](api)
	if err != nil {
		return fmt.Errorf("new verifier: %w", err)
	}
	for i := range a.Proofs {
		if err := verifier.AssertProof(a.VerifyingKey, a.Proofs[i], a.InnerWitnesss[i]); err != nil {
			return fmt.Errorf("asserting proof: %w", err)
		}
	}
	return nil
}

func (a *AggregateCircuit[FR, G1El, G2El, GtEl]) PreCompile(args ...interface{}) {
	innerGroth16List := args[0].([]*Groth16Wrapper)
	a.VerifyingKey = recursion_groth16.PlaceholderVerifyingKey[G1El, G2El, GtEl](innerGroth16List[0].CCS)
	for range innerGroth16List {
		a.Proofs = append(a.Proofs, recursion_groth16.PlaceholderProof[G1El, G2El](innerGroth16List[0].CCS))
		a.InnerWitnesss = append(a.InnerWitnesss, recursion_groth16.PlaceholderWitness[FR](innerGroth16List[0].CCS))
	}
}

func (a *AggregateCircuit[FR, G1El, G2El, GtEl]) Assign(curveName string, args ...interface{}) {
	innerGroth16List := args[0].([]*Groth16Wrapper)
	circuitVK, err := recursion_groth16.ValueOfVerifyingKey[G1El, G2El, GtEl](innerGroth16List[0].VK)
	if err != nil {
		logger.Fatal("initializing verifying key failed, error: %s", err)
	}
	a.VerifyingKey = circuitVK
	for _, innerGroth16 := range innerGroth16List {
		circuitProof, err := recursion_groth16.ValueOfProof[G1El, G2El](innerGroth16.Proof)
		if err != nil {
			logger.Fatal("initializing proof failed, error: %s", err)
		}
		a.Proofs = append(a.Proofs, circuitProof)

		circuitWitness, err := recursion_groth16.ValueOfWitness[FR](innerGroth16.WitnessFull)
		if err != nil {
			logger.Fatal("initializing witness failed, error: %s", err)
		}
		a.InnerWitnesss = append(a.InnerWitnesss, circuitWitness)
	}
}

type AggregateCircuitConstant[FR emulated.FieldParams, G1El algebra.G1ElementT, G2El algebra.G2ElementT, GtEl algebra.GtElementT] struct {
	Proofs        []recursion_groth16.Proof[G1El, G2El]
	vk            recursion_groth16.VerifyingKey[G1El, G2El, GtEl] `gnark:"-"`
	InnerWitnesss []recursion_groth16.Witness[FR]
}

func (a *AggregateCircuitConstant[FR, G1El, G2El, GtEl]) Define(api frontend.API) error {
	verifier, err := recursion_groth16.NewVerifier[FR, G1El, G2El, GtEl](api)
	if err != nil {
		return fmt.Errorf("new verifier: %w", err)
	}
	for i := range a.Proofs {
		if err := verifier.AssertProof(a.vk, a.Proofs[i], a.InnerWitnesss[i]); err != nil {
			return fmt.Errorf("asserting proof: %w", err)
		}
	}
	return nil
}

func (a *AggregateCircuitConstant[FR, G1El, G2El, GtEl]) PreCompile(args ...interface{}) {
	innerGroth16List := args[0].([]*Groth16Wrapper)
	circuitVK, err := recursion_groth16.ValueOfVerifyingKey[G1El, G2El, GtEl](innerGroth16List[0].VK)
	if err != nil {
		logger.Fatal("initializing verifying key failed, error: %s", err)
	}
	a.vk = circuitVK
	for range innerGroth16List {
		a.Proofs = append(a.Proofs, recursion_groth16.PlaceholderProof[G1El, G2El](innerGroth16List[0].CCS))
		a.InnerWitnesss = append(a.InnerWitnesss, recursion_groth16.PlaceholderWitness[FR](innerGroth16List[0].CCS))
	}
}

func (a *AggregateCircuitConstant[FR, G1El, G2El, GtEl]) Assign(curveName string, args ...interface{}) {
	innerGroth16List := args[0].([]*Groth16Wrapper)
	for i := range innerGroth16List {
		circuitProof, err := recursion_groth16.ValueOfProof[G1El, G2El](innerGroth16List[i].Proof)
		if err != nil {
			logger.Fatal("initializing proof failed, error: %s", err)
		}
		a.Proofs = append(a.Proofs, circuitProof)

		circuitWitness, err := recursion_groth16.ValueOfWitness[FR](innerGroth16List[i].WitnessFull)
		if err != nil {
			logger.Fatal("initializing witness failed, error: %s", err)
		}
		a.InnerWitnesss = append(a.InnerWitnesss, circuitWitness)
	}
}
