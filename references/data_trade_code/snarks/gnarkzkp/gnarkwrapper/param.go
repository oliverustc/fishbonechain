package gnarkwrapper

import "github.com/consensys/gnark-crypto/ecc"

// CurveMap 定义了曲线名称到曲线ID的映射关系
var CurveMap = map[string]ecc.ID{
	"BN254":     ecc.BN254,
	"BLS12-377": ecc.BLS12_377,
	"BLS12-381": ecc.BLS12_381,
	"BW6-761":   ecc.BW6_761,
	"BW6-633":   ecc.BW6_633,
	"BLS24-315": ecc.BLS24_315,
	"BLS24-317": ecc.BLS24_317,
}

var CurveList = []string{"BN254", "BLS12-377", "BLS12-381", "BW6-761", "BLS24-315", "BW6-633", "BLS24-317"}

var RecursionCurveList = []string{"BN254", "BLS12-377", "BW6-761"}

// ZkDuration 记录零知识证明各阶段的性能指标
type ZkDuration struct {
	CurveName     string // 使用的曲线名称
	CompileTime   int64  // 编译电路所需时间(ns)
	SetupTime     int64  // 设置电路所需时间(ns)
	ConstraintNum int    // 约束数量
	ProveTime     int64  // 生成证明所需时间(ns)
	VerifyTime    int64  // 验证证明所需时间(ns)
}
