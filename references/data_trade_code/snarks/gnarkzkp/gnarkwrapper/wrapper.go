package gnarkwrapper

import (
	"gnarkabc/logger"
	"math/big"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
)

const FpSize = 4 * 8

type CircuitWrapper interface {
	frontend.Circuit
	PreCompile(params ...interface{})
	Assign(curveName string, params ...interface{})
}

// GnarkWrapper 定义了gnark零知识证明方案的通用接口
type GnarkWrapper interface {
	Compile()                                      // 编译电路
	Setup()                                        // 设置证明系统
	GenerateWitness(public bool)                   // 生成见证者
	Prove()                                        // 生成证明
	Verify()                                       // 验证证明
	BenchmarkCompile(iterations int) time.Duration // 编译基准测试
	BenchmarkSetup(iterations int) time.Duration   // 设置基准测试
	BenchmarkProve(iterations int) time.Duration   // 证明基准测试
	BenchmarkVerify(iterations int) time.Duration  // 验证基准测试

	SetAssignment(assignment frontend.Circuit) // 设置电路赋值
	GetConstraintNum() int                     // 获取约束数量
	GetWitnessJson(public bool) []byte         // 获取witness in json

	ExportSolidity(filePath string)     // 导出solidity代码
	ProofMarshall() string              // 证明序列化
	PublicWitnessMarshall() string      // 公开输入序列化
	GetPublicInputNum() string          // 获取公开输入数量
	GenSolProofParams() string          // 生成solidity的proof参数
	GenSolInputParams() string          // 生成solidity的input参数
	SolCompileAndABIgen(solPath string) // 编译和ABI生成
	SolGenMain()                        // 生成main
	SolGenGoMod()                       // 生成go.mod
	SolVerify()                         // 验证solidity证明

	WriteCCS(filePath string)                  // 写入约束系统
	ReadCCS(filePath string)                   // 读取约束系统
	WritePK(filePath string)                   // 写入证明密钥
	ReadPK(filePath string)                    // 读取证明密钥
	WriteVK(filePath string)                   // 写入验证密钥
	ReadVK(filePath string)                    // 读取验证密钥
	WriteWitness(filePath string, public bool) // 写入见证者
	ReadWitness(filePath string, public bool)  // 读取见证者
	WriteProof(filePath string)                // 写入证明
	ReadProof(filePath string)                 // 读取证明

}

// BaseWrapper 提供了零知识证明方案的基础实现
type BaseWrapper struct {
	Circuit       frontend.Circuit            // 电路实例
	Curve         ecc.ID                      // 使用的曲线
	Field         *big.Int                    // 标量域
	Assignment    frontend.Circuit            // 电路赋值
	WitnessFull   witness.Witness             // 完整见证者
	WitnessPublic witness.Witness             // 公开见证者
	CCS           constraint.ConstraintSystem // 约束系统

	CompileTime   time.Duration // 编译时间
	SetupTime     time.Duration // 设置时间
	ProveTime     time.Duration // 证明时间
	VerifyTime    time.Duration // 验证时间
	ConstraintNum int           // 约束数量
}

// NewGnarkWrapper 创建新的零知识证明包装器
// scheme: 证明方案("groth16"或"plonk")
// circuit: 电路实例
// curve: 使用的曲线
func NewGnarkWrapper(scheme string, circuit frontend.Circuit, curve ecc.ID) GnarkWrapper {
	base := BaseWrapper{
		Circuit: circuit,
		Curve:   curve,
		Field:   curve.ScalarField(),
	}

	switch scheme {
	case "groth16":
		return &Groth16Wrapper{
			BaseWrapper: base,
		}
	case "plonk":
		return &PlonkWrapper{
			BaseWrapper: base,
		}
	default:
		panic("unsupported scheme")
	}
}

func ZKP(scheme string, curve ecc.ID, circuit CircuitWrapper, assign CircuitWrapper) (int64, int, int64) {
	zk := NewGnarkWrapper(scheme, circuit, curve)
	zk.Compile()
	zk.Setup()
	zk.SetAssignment(assign)
	zk.Prove()
	zk.Verify()

	zk.BenchmarkProve(10)
	zk.BenchmarkVerify(10)

	var baseWrapper *BaseWrapper
	switch v := zk.(type) {
	case *Groth16Wrapper:
		baseWrapper = &v.BaseWrapper
	case *PlonkWrapper:
		baseWrapper = &v.BaseWrapper
	default:
		logger.Fatal("unsupported scheme type")
	}
	return baseWrapper.ProveTime.Milliseconds(),
		baseWrapper.ConstraintNum,
		baseWrapper.VerifyTime.Milliseconds()
}
