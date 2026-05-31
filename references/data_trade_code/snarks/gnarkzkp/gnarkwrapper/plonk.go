package gnarkwrapper

import (
	"errors"
	"gnarkabc/logger"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/kzg"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/constraint"
	bls12_377cs "github.com/consensys/gnark/constraint/bls12-377"
	bls12_381cs "github.com/consensys/gnark/constraint/bls12-381"
	bls24_315cs "github.com/consensys/gnark/constraint/bls24-315"
	bls24_317cs "github.com/consensys/gnark/constraint/bls24-317"
	bn254cs "github.com/consensys/gnark/constraint/bn254"
	bw6_633cs "github.com/consensys/gnark/constraint/bw6-633"
	bw6_761cs "github.com/consensys/gnark/constraint/bw6-761"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test/unsafekzg"
)

// PlonkWrapper PLONK证明系统的包装器
type PlonkWrapper struct {
	BaseWrapper
	PK    plonk.ProvingKey   // 证明密钥/
	VK    plonk.VerifyingKey // 验证密钥
	Proof plonk.Proof        // 生成的证明
}

// NewPlonk 创建新的PLONK包装器实例
func NewPlonk(circuit frontend.Circuit, curve ecc.ID) *PlonkWrapper {
	return &PlonkWrapper{
		BaseWrapper: BaseWrapper{
			Circuit: circuit,
			Curve:   curve,
			Field:   curve.ScalarField(),
		},
	}
}

// Compile 编译电路
func (p *PlonkWrapper) Compile() {
	logger.Debug("compiling circuit ...")
	var err error
	start := time.Now()
	p.CCS, err = frontend.Compile(p.Field, scs.NewBuilder, p.Circuit)
	if err != nil {
		logger.Fatal("compile circuit failed. " + err.Error())
	}
	p.CompileTime = time.Since(start)
	logger.Debug("circuit compiled, took: " + p.CompileTime.String())
	if p.ConstraintNum == 0 {
		p.ConstraintNum = p.CCS.GetNbConstraints()
		logger.Debug("constraint number: %d", p.ConstraintNum)
	}
}

// Setup 设置电路的证明系统
func (p *PlonkWrapper) Setup() {
	logger.Debug("setting up circuit ...")
	var srs, srsLagrange kzg.SRS
	var err error
	start := time.Now()

	// 提取 SRS 创建逻辑，避免代码重复
	srs, srsLagrange, err = p.createSRS(p.CCS)
	if err != nil {
		logger.Fatal("create SRS failed. " + err.Error())
	}

	p.PK, p.VK, err = plonk.Setup(p.CCS, srs, srsLagrange)
	if err != nil {
		logger.Fatal("setup circuit failed. " + err.Error())
	}

	p.SetupTime = time.Since(start)
	logger.Debug("circuit setup, took: " + p.SetupTime.String())
}

// createSRS 创建结构化参考字符串(SRS)
func (p *PlonkWrapper) createSRS(scs constraint.ConstraintSystem) (kzg.SRS, kzg.SRS, error) {
	switch p.Curve {
	case ecc.BN254:
		return unsafekzg.NewSRS(scs.(*bn254cs.SparseR1CS))
	case ecc.BLS12_377:
		return unsafekzg.NewSRS(scs.(*bls12_377cs.SparseR1CS))
	case ecc.BLS12_381:
		return unsafekzg.NewSRS(scs.(*bls12_381cs.SparseR1CS))
	case ecc.BW6_761:
		return unsafekzg.NewSRS(scs.(*bw6_761cs.SparseR1CS))
	case ecc.BLS24_315:
		return unsafekzg.NewSRS(scs.(*bls24_315cs.SparseR1CS))
	case ecc.BW6_633:
		return unsafekzg.NewSRS(scs.(*bw6_633cs.SparseR1CS))
	case ecc.BLS24_317:
		return unsafekzg.NewSRS(scs.(*bls24_317cs.SparseR1CS))
	}
	return nil, nil, errors.New("invalid curve ID")
}

// SetAssignment 设置电路的赋值
func (p *PlonkWrapper) SetAssignment(assignment frontend.Circuit) {
	p.Assignment = assignment
}

func (p *PlonkWrapper) GenerateWitness(public bool) {
	var err error
	if public {
		if p.WitnessFull != nil {
			p.WitnessPublic, err = p.WitnessFull.Public()
			if err != nil {
				logger.Fatal("generate public witness from witnessfull failed. " + err.Error())
			}
		} else {
			if p.Assignment == nil {
				logger.Fatal("assignment is nil")
			}
			p.WitnessPublic, err = frontend.NewWitness(p.Assignment, p.Field, frontend.PublicOnly())
			if err != nil {
				logger.Fatal("generate public witness from assignment failed. " + err.Error())
			}
		}
	} else {
		if p.Assignment == nil {
			logger.Fatal("assignment is nil")
		}
		p.WitnessFull, err = frontend.NewWitness(p.Assignment, p.Field)
		if err != nil {
			logger.Fatal("generate full witness failed. " + err.Error())
		}
	}

}

// Prove 生成零知识证明
func (p *PlonkWrapper) Prove() {
	logger.Debug("proving circuit ...")
	var err error
	if p.WitnessFull == nil {
		p.GenerateWitness(false)
	}
	start := time.Now()
	p.Proof, err = plonk.Prove(p.CCS, p.PK, p.WitnessFull)
	if err != nil {
		logger.Fatal("prove circuit failed. " + err.Error())
	}
	p.ProveTime = time.Since(start)
	logger.Debug("circuit proved, took: " + p.ProveTime.String())
}

// Verify 验证零知识证明
func (p *PlonkWrapper) Verify() {
	logger.Debug("verifying circuit ...")
	var err error
	if p.WitnessPublic == nil {
		p.WitnessPublic, err = frontend.NewWitness(p.Assignment, p.Field, frontend.PublicOnly())
		if err != nil {
			logger.Fatal("generate public witness failed. " + err.Error())
		}
	}
	start := time.Now()
	err = plonk.Verify(p.Proof, p.VK, p.WitnessPublic)
	if err != nil {
		logger.Fatal("verify circuit failed. " + err.Error())
	}
	p.VerifyTime = time.Since(start)
	logger.Debug("circuit verified, took: " + p.VerifyTime.String())
}

// BenchmarkCompile 对编译过程进行基准测试
func (p *PlonkWrapper) BenchmarkCompile(iterations int) time.Duration {
	logger.Debug("benchmarking compile circuit ...")
	var compileTime time.Duration
	for i := 0; i < iterations; i++ {
		p.Compile()
		compileTime += p.CompileTime
	}
	p.CompileTime = compileTime / time.Duration(iterations)
	logger.Debug("after %d iterations, compile time: %s", iterations, p.CompileTime.String())
	return p.CompileTime
}

// BenchmarkSetup 对设置过程进行基准测试
func (p *PlonkWrapper) BenchmarkSetup(iterations int) time.Duration {
	logger.Debug("benchmarking setup ")
	var setupTime time.Duration
	for i := 0; i < iterations; i++ {
		p.Setup()
		setupTime += p.SetupTime
	}
	p.SetupTime = setupTime / time.Duration(iterations)
	logger.Debug("after %d iterations, setup time: %s", iterations, p.SetupTime.String())
	return p.SetupTime
}

// BenchmarkProve 对证明生成过程进行基准测试
func (p *PlonkWrapper) BenchmarkProve(iterations int) time.Duration {
	logger.Debug("benchmarking proving circuit ...")
	var proveTime time.Duration
	for i := 0; i < iterations; i++ {
		p.Prove()
		proveTime += p.ProveTime
	}
	p.ProveTime = proveTime / time.Duration(iterations)
	logger.Debug("after %d iterations, prove time: %s", iterations, p.ProveTime.String())
	return p.ProveTime
}

// BenchmarkVerify 对验证过程进行基准测试
func (p *PlonkWrapper) BenchmarkVerify(iterations int) time.Duration {
	logger.Debug("benchmarking verifying circuit ...")
	var verifyTime time.Duration
	for i := 0; i < iterations; i++ {
		p.Verify()
		verifyTime += p.VerifyTime
	}
	p.VerifyTime = verifyTime / time.Duration(iterations)
	logger.Debug("after %d iterations, verify time: %s", iterations, p.VerifyTime.String())
	return p.VerifyTime
}

func (p *PlonkWrapper) GetConstraintNum() int {
	return p.CCS.GetNbConstraints()
}

func (p *PlonkWrapper) GetWitnessJson(public bool) []byte {
	schama, err := frontend.NewSchema(p.Assignment)
	if err != nil {
		logger.Fatal("get schema failed: %v", err)
	}
	if public {
		witness, err := p.WitnessFull.Public()
		if err != nil {
			logger.Fatal("get public witness failed: %v", err)
		}
		witnessJson, err := witness.ToJSON(schama)
		if err != nil {
			logger.Fatal("get public witness json failed: %v", err)
		}
		return witnessJson
	} else {
		witnessJson, err := p.WitnessFull.ToJSON(schama)
		if err != nil {
			logger.Fatal("get witness json failed: %v", err)
		}
		return witnessJson
	}
}
