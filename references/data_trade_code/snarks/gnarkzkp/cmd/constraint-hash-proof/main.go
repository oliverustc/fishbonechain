package main

import (
	"flag"
	"fmt"
	"gnarkabc/utils"
	"os"
	"strconv"
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
	circuitType := "range"
	mode := ""
	schemeStr := "groth16,plonk"
	num := 20

	// 解析位置参数
	if len(args) > 0 {
		circuitType = args[0]
	}
	if len(args) > 1 {
		mode = args[1]
	}
	if len(args) > 2 {
		schemeStr = args[2]
	}
	if len(args) > 3 {
		var err error
		n, err := strconv.Atoi(args[3])
		if err == nil {
			num = n
		} else {
			fmt.Printf("警告: 证明数量参数无效 '%s'，使用默认值 20\n", args[3])
		}
	}

	// 显示帮助信息
	if *help || circuitType == "" || mode == "" {
		fmt.Println("用法: ./程序名 [电路类型] [操作类型] [方案列表] [证明数量]")
		fmt.Println("电路类型: range, subset")
		fmt.Println("操作类型: gen, prove, verify")
		fmt.Println("示例:")
		fmt.Println("  生成 range 证明:  ./程序名 range gen groth16,plonk 20")
		fmt.Println("  执行 subset 证明:  ./程序名 subset prove groth16 10")
		fmt.Println("  验证 range 证明:  ./程序名 range verify plonk 5")
		os.Exit(0)
	}

	// 解析方案列表
	schemes := splitAndTrim(schemeStr, ",")
	if len(schemes) == 0 {
		schemes = []string{"groth16", "plonk"}
	}

	// 确保目录存在
	utils.EnsureDirExists("output")

	// 根据电路类型和操作模式执行相应的功能
	switch circuitType {
	case "range":
		// Range Hash 电路
		switch mode {
		case "gen":
			// 生成证明
			for _, s := range schemes {
				GenRangeHashZKP(s, num)
			}
		case "prove":
			// 执行证明
			for _, s := range schemes {
				ProveRangeHashZKP(s, num)
			}
		case "verify":
			// 验证证明
			for _, s := range schemes {
				VerifyRangeHashZKP(s, num)
			}
		case "sol":
			// 导出 Solidity 证明
			for _, s := range schemes {
				RangeHashProofExportSolidity(s)
			}
		default:
			fmt.Printf("未知的操作模式: %s\n", mode)
			os.Exit(1)
		}
	case "subset":
		// Subset Hash 电路
		switch mode {
		case "gen":
			// 生成证明
			for _, s := range schemes {
				GenSubsetHashZKP(s, num)
			}
		case "prove":
			// 执行证明
			for _, s := range schemes {
				ProveSubsetHashZKP(s, num)
			}
		case "verify":
			// 验证证明
			for _, s := range schemes {
				VerifySubsetHashZKP(s, num)
			}
		case "sol":
			// 导出 Solidity 证明
			for _, s := range schemes {
				SubsetHashProofExportSolidity(s)
			}
		default:
			fmt.Printf("未知的操作模式: %s\n", mode)
			os.Exit(1)
		}
	case "substr":
		// Substr Hash 电路
		switch mode {
		case "gen":
			// 生成证明
			for _, s := range schemes {
				GenSubstrHashZKP(s, num)
			}
		case "prove":
			// 执行证明
			for _, s := range schemes {
				ProveSubstrHashZKP(s, num)
			}
		case "verify":
			// 验证证明
			for _, s := range schemes {
				VerifySubstrHashZKP(s, num)
			}
		case "sol":
			// 导出 Solidity 证明
			for _, s := range schemes {
				SubstrHashProofExportSolidity(s)
			}
		default:
			fmt.Printf("未知的操作模式: %s\n", mode)
			os.Exit(1)
		}
	default:
		fmt.Printf("未知的电路类型: %s\n", circuitType)
		os.Exit(1)
	}
}
