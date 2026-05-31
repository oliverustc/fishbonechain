package gnarkwrapper

import (
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"

	"gnarkabc/logger"
)

// Groth16Wrapper Groth16证明系统的包装器
type Groth16Wrapper struct {
	BaseWrapper
	PK    groth16.ProvingKey   // 证明密钥
	VK    groth16.VerifyingKey // 验证密钥
	Proof groth16.Proof        // 生成的证明
}

// NewGroth16 创建新的Groth16包装器实例
func NewGroth16(circuit frontend.Circuit, curve ecc.ID) *Groth16Wrapper {
	return &Groth16Wrapper{
		BaseWrapper: BaseWrapper{
			Circuit: circuit,
			Curve:   curve,
			Field:   curve.ScalarField(),
		},
	}
}

// Compile 编译电路
func (g *Groth16Wrapper) Compile() {
	logger.Debug("compiling circuit ...")
	var err error
	start := time.Now()
	g.CCS, err = frontend.Compile(g.Field, r1cs.NewBuilder, g.Circuit)
	if err != nil {
		logger.Fatal("compile circuit failed. " + err.Error())
	}
	g.CompileTime = time.Since(start)
	logger.Debug("circuit compiled, took: " + g.CompileTime.String())
	if g.ConstraintNum == 0 {
		g.ConstraintNum = g.CCS.GetNbConstraints()
		logger.Debug("constraint number: %d", g.ConstraintNum)
	}
}

// Setup 设置电路的证明系统
func (g *Groth16Wrapper) Setup() {
	logger.Debug("setting up circuit ...")
	var err error
	start := time.Now()
	g.PK, g.VK, err = groth16.Setup(g.CCS)
	if err != nil {
		logger.Fatal("setup circuit failed. " + err.Error())
	}
	g.SetupTime = time.Since(start)
	logger.Debug("circuit setup, took: " + g.SetupTime.String())
}

// generateWitness 生成见证者数据
// publicOnly: 是否只生成公开输入的见证者
func (g *Groth16Wrapper) GenerateWitness(publicOnly bool) {
	var err error
	if publicOnly {
		if g.WitnessFull != nil {
			g.WitnessPublic, err = g.WitnessFull.Public()
			if err != nil {
				logger.Fatal("generate public witness from witnessfull failed. " + err.Error())
			}
		} else {
			if g.Assignment == nil {
				logger.Fatal("assignment is nil")
			}
			g.WitnessPublic, err = frontend.NewWitness(g.Assignment, g.Field, frontend.PublicOnly())
			if err != nil {
				logger.Fatal("generate public witness from assignment failed. " + err.Error())
			}
		}
	} else {
		if g.Assignment == nil {
			logger.Fatal("assignment is nil")
		}
		g.WitnessFull, err = frontend.NewWitness(g.Assignment, g.Field)
		if err != nil {
			logger.Fatal("generate full witness failed. " + err.Error())
		}
	}
}

// SetAssignment 设置电路的赋值
func (g *Groth16Wrapper) SetAssignment(assignment frontend.Circuit) {
	g.Assignment = assignment
}

// Prove 生成零知识证明
func (g *Groth16Wrapper) Prove() {
	logger.Debug("proving ...")
	var err error
	if g.WitnessFull == nil {
		g.GenerateWitness(false)
	}
	start := time.Now()
	g.Proof, err = groth16.Prove(g.CCS, g.PK, g.WitnessFull)
	if err != nil {
		logger.Fatal("prove failed. " + err.Error())
	}
	g.ProveTime = time.Since(start)
	logger.Debug("circuit proved, took: " + g.ProveTime.String())
}

// Verify 验证零知识证明
func (g *Groth16Wrapper) Verify() {
	logger.Debug("verifying ...")
	var err error
	if g.WitnessPublic == nil {
		g.GenerateWitness(true)
	}
	start := time.Now()
	err = groth16.Verify(g.Proof, g.VK, g.WitnessPublic)
	if err != nil {
		logger.Fatal("verify proof failed.")
	} else {
		logger.Debug("circuit verified")
	}
	g.VerifyTime = time.Since(start)
	logger.Debug("circuit verified, took: " + g.VerifyTime.String())
}

// BenchmarkCompile 对编译过程进行基准测试
func (g *Groth16Wrapper) BenchmarkCompile(iterations int) time.Duration {
	logger.Debug("benchmarking compiling circuit ...")
	var compileTime time.Duration
	for i := 0; i < iterations; i++ {
		g.Compile()
		compileTime += g.CompileTime
	}
	g.CompileTime = compileTime / time.Duration(iterations)
	logger.Debug("after %d iterations, compile time: %s", iterations, g.CompileTime.String())
	return g.CompileTime
}

// BenchmarkSetup 对设置过程进行基准测试
func (g *Groth16Wrapper) BenchmarkSetup(iterations int) time.Duration {
	logger.Debug("benchmarking setup circuit ...")
	var setupTime time.Duration
	for i := 0; i < iterations; i++ {
		g.Setup()
		setupTime += g.SetupTime
	}
	g.SetupTime = setupTime / time.Duration(iterations)
	logger.Debug("after %d iterations, setup time: %s", iterations, g.SetupTime.String())
	return g.SetupTime
}

// BenchmarkProve 对证明生成过程进行基准测试
func (g *Groth16Wrapper) BenchmarkProve(iterations int) time.Duration {
	logger.Debug("benchmarking proving circuit ...")
	var proveTime time.Duration
	for i := 0; i < iterations; i++ {
		g.Prove()
		proveTime += g.ProveTime
	}
	g.ProveTime = proveTime / time.Duration(iterations)
	logger.Debug("after %d iterations, prove time: %s", iterations, g.ProveTime.String())
	return g.ProveTime
}

// BenchmarkVerify 对验证过程进行基准测试
func (g *Groth16Wrapper) BenchmarkVerify(iterations int) time.Duration {
	logger.Debug("benchmarking verifying circuit ...")
	var verifyTime time.Duration
	for i := 0; i < iterations; i++ {
		g.Verify()
		verifyTime += g.VerifyTime
	}
	g.VerifyTime = verifyTime / time.Duration(iterations)
	logger.Debug("after %d iterations, verify time: %s", iterations, g.VerifyTime.String())
	return g.VerifyTime
}

// 获取电路中约束数量
func (g *Groth16Wrapper) GetConstraintNum() int {
	return g.CCS.GetNbConstraints()
}

// 获取json格式witness
func (g *Groth16Wrapper) GetWitnessJson(public bool) []byte {
	schama, err := frontend.NewSchema(g.Assignment)
	if err != nil {
		logger.Fatal("get schema failed: %v", err)
	}
	if public {
		witness, err := g.WitnessFull.Public()
		if err != nil {
			logger.Fatal("get public witness failed: %v", err)
		}
		witnessJson, err := witness.ToJSON(schama)
		if err != nil {
			logger.Fatal("get public witness json failed: %v", err)
		}
		return witnessJson
	} else {
		witnessJson, err := g.WitnessFull.ToJSON(schama)
		if err != nil {
			logger.Fatal("get witness json failed: %v", err)
		}
		return witnessJson
	}
}
