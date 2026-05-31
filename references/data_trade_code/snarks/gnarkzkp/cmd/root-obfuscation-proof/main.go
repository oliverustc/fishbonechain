package main

import (
	"flag"
	"fmt"
	"gnarkabc/utils"
	"os"
	"strings"
)

// 分割字符串并去除空白
func splitAndTrim(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func main() {
	// 定义命令行参数
	help := flag.Bool("help", false, "显示帮助信息")

	// 解析命令行参数
	flag.Parse()

	// 获取位置参数（非标志参数）
	args := flag.Args()

	// 默认值
	mode := ""
	schemeStr := "groth16,plonk"
	depthList := []int{10, 12, 14, 16, 18, 20}

	// 解析位置参数
	if len(args) > 0 {
		mode = args[0]
	}
	if len(args) > 1 {
		schemeStr = args[1]
	}

	// 显示帮助信息
	if *help || mode == "" {
		fmt.Println("用法: ./程序名 [操作类型] [方案列表]")
		fmt.Println("操作类型: gen, prove, verify")
		fmt.Println("示例:")
		fmt.Println("  生成证明:  ./程序名 gen groth16,plonk")
		fmt.Println("  执行证明:  ./程序名 prove groth16")
		fmt.Println("  验证证明:  ./程序名 verify plonk")
		os.Exit(0)
	}

	// 解析方案列表
	schemes := splitAndTrim(schemeStr, ",")
	if len(schemes) == 0 {
		schemes = []string{"groth16", "plonk"}
	}

	// 确保目录存在
	utils.EnsureDirExists("output")

	// 根据操作模式执行相应的功能
	switch mode {
	case "gen":
		// 生成证明
		for _, s := range schemes {
			GenRootObfuscationZKP(s, depthList)
		}
	case "prove":
		// 执行证明
		for _, s := range schemes {
			ProveRootObfuscationZKP(s, depthList)
		}
	case "verify":
		// 验证证明
		for _, s := range schemes {
			VerifyRootObfuscationZKP(s, depthList)
		}
	case "sol":
		// 导出Solidity证明
		for _, s := range schemes {
			RootObfuscationProofExportSolidity(s, depthList)
		}
	default:
		fmt.Printf("未知的操作模式: %s\n", mode)
		os.Exit(1)
	}
}
