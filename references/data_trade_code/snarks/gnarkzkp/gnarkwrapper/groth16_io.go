package gnarkwrapper

import (
	"gnarkabc/logger"
	"gnarkabc/utils"
	"os"

	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/backend/witness"
)

func (g *Groth16Wrapper) WriteCCS(filePath string) {
	if filePath == "" {
		logger.Debug("CCSfilePath is empty, using default output/ccs")
		filePath = "output/ccs"
	}
	logger.Debug("Writing ccs to " + filePath)
	utils.EnsureDirExists("output")
	file, err := os.Create(filePath)
	if err != nil {
		logger.Error("failed to create file: " + err.Error())
	}
	defer file.Close()
	size, err := g.CCS.WriteTo(file)
	if err != nil {
		logger.Error("failed to write ccs to file: " + err.Error())
	}
	logger.Debug("write ccs to %s, size= %d", filePath, size)
}

func (g *Groth16Wrapper) ReadCCS(filePath string) {
	if filePath == "" {
		logger.Debug("CCS filePath is empty, using default output/ccs")
		filePath = "output/ccs"
	}
	logger.Debug("Reading ccs from " + filePath)
	utils.EnsureDirExists("output")

	// 初始化 CCS
	if g.CCS == nil {
		g.CCS = groth16.NewCS(g.Curve)
	}

	file, err := os.Open(filePath)
	if err != nil {
		logger.Error("failed to open file: " + err.Error())
		return
	}
	defer file.Close()

	size, err := g.CCS.ReadFrom(file)
	if err != nil {
		logger.Error("failed to read ccs from file: " + err.Error())
	}
	logger.Debug("read ccs from %s, size= %d", filePath, size)
}

func (g *Groth16Wrapper) WritePK(filePath string) {
	if filePath == "" {
		logger.Debug("Proving Key filePath is empty, using default output/pk")
		filePath = "output/pk"
	}
	logger.Debug("Writing proving key to " + filePath)
	utils.EnsureDirExists("output")
	file, err := os.Create(filePath)
	if err != nil {
		logger.Error("failed to create file: " + err.Error())
	}
	defer file.Close()
	size, err := g.PK.WriteTo(file)
	if err != nil {
		logger.Error("failed to write pk to file: " + err.Error())
	}
	logger.Debug("wrote proving key to %s, size= %d", filePath, size)
}

func (g *Groth16Wrapper) ReadPK(filePath string) {
	if filePath == "" {
		logger.Debug("Proving Key filePath is empty, using default output/pk")
		filePath = "output/pk"
	}
	logger.Debug("Reading proving key from " + filePath)
	utils.EnsureDirExists("output")
	file, err := os.Open(filePath)
	if err != nil {
		logger.Fatal("open file failed: %v", err)
	}
	defer file.Close()
	if g.PK == nil {
		g.PK = groth16.NewProvingKey(g.Curve)
	}
	size, err := g.PK.ReadFrom(file)
	if err != nil {
		logger.Error("failed to write pk to file: " + err.Error())
	}
	logger.Debug("read proving key from %s, size= %d", filePath, size)
}

func (g *Groth16Wrapper) WriteVK(filePath string) {
	if filePath == "" {
		logger.Debug("Verification Key filePath is empty, using default output/vk")
		filePath = "output/vk"
	}
	logger.Debug("Writing verification key to " + filePath)
	utils.EnsureDirExists("output")
	file, err := os.Create(filePath)
	if err != nil {
		logger.Error("failed to create file: " + err.Error())
	}
	defer file.Close()
	size, err := g.VK.WriteTo(file)
	if err != nil {
		logger.Error("failed to write vk to file: " + err.Error())
	}
	logger.Debug("wrote verification key to %s, size= %d", filePath, size)
}

func (g *Groth16Wrapper) ReadVK(filePath string) {
	if filePath == "" {
		logger.Debug("Verification Key filePath is empty, using default output/vk")
		filePath = "output/vk"
	}
	logger.Debug("Reading verification key from " + filePath)
	utils.EnsureDirExists("output")
	file, err := os.Open(filePath)
	if err != nil {
		logger.Fatal("open file failed: %v", err)
	}
	defer file.Close()
	if g.VK == nil {
		g.VK = groth16.NewVerifyingKey(g.Curve)
	}
	size, err := g.VK.ReadFrom(file)
	if err != nil {
		logger.Error("failed to write vk to file: " + err.Error())
	}
	logger.Debug("read verification key from %s, size= %d", filePath, size)
}

func (g *Groth16Wrapper) WriteWitness(filePath string, public bool) {
	if filePath == "" {
		if public {
			logger.Debug("Witness filePath is empty, using default output/public_witness")
			filePath = "output/public_witness"
		} else {
			logger.Debug("Witness filePath is empty, using default output/witness")
			filePath = "output/witness"
		}
	}
	if public {
		logger.Debug("Writing public witness to " + filePath)
	} else {
		logger.Debug("Writing witness to " + filePath)
	}
	utils.EnsureDirExists("output")
	file, err := os.Create(filePath)
	if err != nil {
		logger.Error("failed to create file: " + err.Error())
	}
	defer file.Close()
	var size int64
	if public {
		size, err = g.WitnessPublic.WriteTo(file)
	} else {
		size, err = g.WitnessFull.WriteTo(file)
	}
	if err != nil {
		logger.Error("failed to write witness to file: " + err.Error())
	}
	if public {
		logger.Debug("wrote public witness to %s, size= %d", filePath, size)
	} else {
		logger.Debug("wrote witness to %s, size= %d", filePath, size)
	}
}

func (g *Groth16Wrapper) ReadWitness(filePath string, public bool) {
	if filePath == "" {
		if public {
			logger.Debug("Witness filePath is empty, using default output/public_witness")
			filePath = "output/public_witness"
		} else {
			logger.Debug("Witness filePath is empty, using default output/witness")
			filePath = "output/witness"
		}
	}
	if public {
		logger.Debug("Reading public witness from " + filePath)
	} else {
		logger.Debug("Reading witness from " + filePath)
	}
	utils.EnsureDirExists("output")
	file, err := os.Open(filePath)
	if err != nil {
		logger.Fatal("open file failed: %v", err)
	}
	defer file.Close()
	var size int64
	if public {
		if g.WitnessPublic == nil {
			g.WitnessPublic, err = witness.New(g.Field)
			if err != nil {
				logger.Fatal("failed to create witness: " + err.Error())
			}
		}
		size, err = g.WitnessPublic.ReadFrom(file)
	} else {
		if g.WitnessFull == nil {
			g.WitnessFull, err = witness.New(g.Field)
			if err != nil {
				logger.Fatal("failed to create witness: " + err.Error())
			}
		}
		size, err = g.WitnessFull.ReadFrom(file)
	}
	if err != nil {
		logger.Error("failed to read witness from file: " + err.Error())
		return
	}
	if public {
		logger.Debug("read public witness from %s, size= %d", filePath, size)
	} else {
		logger.Debug("read witness from %s, size= %d", filePath, size)
	}
}

func (g *Groth16Wrapper) WriteProof(filePath string) {
	if filePath == "" {
		logger.Debug("Proof filePath is empty, using default output/proof")
		filePath = "output/proof"
	}
	logger.Debug("Writing proof to " + filePath)
	utils.EnsureDirExists("output")
	file, err := os.Create(filePath)
	if err != nil {
		logger.Error("failed to create file: " + err.Error())
	}
	defer file.Close()
	size, err := g.Proof.WriteTo(file)
	if err != nil {
		logger.Error("failed to write proof to file: " + err.Error())
	}
	logger.Debug("wrote proof to %s, size= %d", filePath, size)
}

func (g *Groth16Wrapper) ReadProof(filePath string) {
	if filePath == "" {
		logger.Debug("Proof filePath is empty, using default output/proof")
		filePath = "output/proof"
	}
	logger.Debug("Reading proof from " + filePath)
	utils.EnsureDirExists("output")
	file, err := os.Open(filePath)
	if err != nil {
		logger.Fatal("open file failed: %v", err)
	}
	defer file.Close()
	if g.Proof == nil {
		g.Proof = groth16.NewProof(g.Curve)
	}
	size, err := g.Proof.ReadFrom(file)
	if err != nil {
		logger.Error("failed to write proof to file: " + err.Error())
	}
	logger.Debug("read proof from %s, size= %d", filePath, size)
}
