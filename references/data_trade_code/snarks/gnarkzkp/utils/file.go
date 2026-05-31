package utils

import (
	"gnarkabc/logger"
	"os"
)

// 如果文件不存在，则创建文件
func EnsureFileExists(filePath string) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		file, err := os.Create(filePath)
		if err != nil {
			logger.ConsoleLog.Fatal(err)
		}
		file.Close()
	}
}

// 如果文件不存在，则创建文件夹
func EnsureDirExists(dirPath string) {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			logger.ConsoleLog.Fatal(err)
		}
	}
}

func RemoveFile(filePath string) error {
	// 如果文件不存在，则输出跳过
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		logger.ConsoleLog.Warnf("文件 %s 不存在，跳过删除", filePath)
		return nil
	}
	logger.ConsoleLog.Debugf("删除文件 %s", filePath)
	return os.Remove(filePath)
}

func RemoveDir(dirPath string) error {
	// 如果文件夹不存在，则输出跳过
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		logger.ConsoleLog.Warnf("文件夹 %s 不存在，跳过删除", dirPath)
		return nil
	}
	logger.ConsoleLog.Debugf("删除文件夹 %s", dirPath)
	return os.RemoveAll(dirPath)
}
