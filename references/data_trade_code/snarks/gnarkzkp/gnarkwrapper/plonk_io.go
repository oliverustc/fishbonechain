package gnarkwrapper

import (
	"gnarkabc/logger"
	"gnarkabc/utils"
	"os"

	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/backend/witness"
)

func (p *PlonkWrapper) WriteCCS(filePath string) {
	if filePath == "" {
		logger.Debug("CCS filePath is empty, using default output/ccs")
		filePath = "output/ccs"
	}
	logger.Debug("Writing ccs to " + filePath)
	utils.EnsureDirExists("output")
	file, err := os.Create(filePath)
	if err != nil {
		logger.Error("failed to create file: " + err.Error())
	}
	defer file.Close()
	size, err := p.CCS.WriteTo(file)
	if err != nil {
		logger.Error("failed to write ccs to file: " + err.Error())
	}
	logger.Debug("write ccs to %s, size= %d", filePath, size)
}

func (p *PlonkWrapper) ReadCCS(filePath string) {
	if filePath == "" {
		logger.Debug("CCS filePath is empty, using default output/ccs")
		filePath = "output/ccs"
	}
	logger.Debug("Reading ccs from " + filePath)
	utils.EnsureDirExists("output")
	file, err := os.Open(filePath)
	if err != nil {
		logger.Error("failed to open file: " + err.Error())
		return
	}
	defer file.Close()
	if p.CCS == nil {
		p.CCS = plonk.NewCS(p.Curve)
	}
	size, err := p.CCS.ReadFrom(file)
	if err != nil {
		logger.Error("failed to read ccs from file: " + err.Error())
	}
	logger.Debug("read ccs from %s, size= %d", filePath, size)
}

func (p *PlonkWrapper) WritePK(filePath string) {
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
	size, err := p.PK.WriteTo(file)
	if err != nil {
		logger.Error("failed to write pk to file: " + err.Error())
	}
	logger.Debug("wrote proving key to %s, size= %d", filePath, size)
}

func (p *PlonkWrapper) ReadPK(filePath string) {
	if filePath == "" {
		logger.Debug("Proving Key filePath is empty, using default output/pk")
		filePath = "output/pk"
	}
	logger.Debug("Reading proving key from " + filePath)
	utils.EnsureDirExists("output")
	file, err := os.Open(filePath)
	if err != nil {
		logger.Error("failed to open file: " + err.Error())
		return
	}
	defer file.Close()
	if p.PK == nil {
		p.PK = plonk.NewProvingKey(p.Curve)
	}
	size, err := p.PK.ReadFrom(file)
	if err != nil {
		logger.Error("failed to read pk from file: " + err.Error())
	}
	logger.Debug("read proving key from %s, size= %d", filePath, size)
}

func (p *PlonkWrapper) WriteVK(filePath string) {
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
	size, err := p.VK.WriteTo(file)
	if err != nil {
		logger.Error("failed to write vk to file: " + err.Error())
	}
	logger.Debug("wrote verification key to %s, size= %d", filePath, size)
}

func (p *PlonkWrapper) ReadVK(filePath string) {
	if filePath == "" {
		logger.Debug("Verification Key filePath is empty, using default output/vk")
		filePath = "output/vk"
	}
	logger.Debug("Reading verification key from " + filePath)
	utils.EnsureDirExists("output")
	file, err := os.Open(filePath)
	if err != nil {
		logger.Error("failed to open file: " + err.Error())
		return
	}
	defer file.Close()
	if p.VK == nil {
		p.VK = plonk.NewVerifyingKey(p.Curve)
	}
	size, err := p.VK.ReadFrom(file)
	if err != nil {
		logger.Error("failed to read vk from file: " + err.Error())
	}
	logger.Debug("read verification key from %s, size= %d", filePath, size)
}

func (p *PlonkWrapper) WriteWitness(filePath string, public bool) {
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
		size, err = p.WitnessPublic.WriteTo(file)
	} else {
		size, err = p.WitnessFull.WriteTo(file)
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

func (p *PlonkWrapper) ReadWitness(filePath string, public bool) {
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
		logger.Error("failed to open file: " + err.Error())
		return
	}
	defer file.Close()
	var size int64
	if public {
		if p.WitnessPublic == nil {
			p.WitnessPublic, err = witness.New(p.Field)
			if err != nil {
				logger.Fatal("failed to create witness: " + err.Error())
			}
		}
		size, err = p.WitnessPublic.ReadFrom(file)
	} else {
		if p.WitnessFull == nil {
			p.WitnessFull, err = witness.New(p.Field)
			if err != nil {
				logger.Fatal("failed to create witness: " + err.Error())
			}
		}
		size, err = p.WitnessFull.ReadFrom(file)
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

func (p *PlonkWrapper) WriteProof(filePath string) {
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
	size, err := p.Proof.WriteTo(file)
	if err != nil {
		logger.Error("failed to write proof to file: " + err.Error())
	}
	logger.Debug("wrote proof to %s, size= %d", filePath, size)
}

func (p *PlonkWrapper) ReadProof(filePath string) {
	if filePath == "" {
		logger.Debug("Proof filePath is empty, using default output/proof")
		filePath = "output/proof"
	}
	logger.Debug("Reading proof from " + filePath)
	utils.EnsureDirExists("output")
	file, err := os.Open(filePath)
	if err != nil {
		logger.Error("failed to open file: " + err.Error())
		return
	}
	defer file.Close()
	if p.Proof == nil {
		p.Proof = plonk.NewProof(p.Curve)
	}
	size, err := p.Proof.ReadFrom(file)
	if err != nil {
		logger.Error("failed to read proof from file: " + err.Error())
	}
	logger.Debug("read proof from %s, size= %d", filePath, size)
}
