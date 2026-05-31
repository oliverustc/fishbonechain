package piano

import (
	"crypto/rand"
	"errors"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/fft"
	"github.com/oliverustc/bpiano/dkzg"
)

// Setup 为 Piano 协议生成 ProvingKey 和 VerifyingKey。
//
// ci 描述共享电路结构（选择子 + 置换）。
// M 是子节点数量（须为 2 的幂且 ≥ 1）。
// T 是 X 轴域的行数（须为 2 的幂且 ≥ ci 编码的行数）。
//
// SRS 陷门在内部随机生成，SRS 构建完毕后立即丢弃。
// 如需确定性测试，使用 SetupWithTrapdoors。
func Setup(ci CircuitInfo, M, T uint64) (*ProvingKey, *VerifyingKey, error) {
	// 随机生成陷门。
	ord := new(big.Int)
	fr := new(big.Int)
	// 获取 BN254 标量域阶（来自 fr.Modulus()）。
	{
		var tmp [32]byte
		if _, err := rand.Read(tmp[:]); err != nil {
			return nil, nil, err
		}
	}
	scalarField := new(big.Int)
	{
		var e [32]byte
		scalarField.SetBytes(e[:]) // 下方覆盖
	}
	// 从虚拟 fr.Element 获取曲线标量域阶。
	{
		var dummy dummyFr
		scalarField.Set(dummy.Modulus())
		_ = ord
		_ = fr
	}

	tauX, err := rand.Int(rand.Reader, scalarField)
	if err != nil {
		return nil, nil, err
	}
	tauY, err := rand.Int(rand.Reader, scalarField)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		tauX.SetInt64(0)
		tauY.SetInt64(0)
	}()

	return setupWithTrapdoors(ci, M, T, tauX, tauY)
}

// SetupWithTrapdoors 与 Setup 相同，但接受显式陷门参数，仅用于测试。
func SetupWithTrapdoors(ci CircuitInfo, M, T uint64, tauX, tauY *big.Int) (*ProvingKey, *VerifyingKey, error) {
	return setupWithTrapdoors(ci, M, T, tauX, tauY)
}

func setupWithTrapdoors(ci CircuitInfo, M, T uint64, tauX, tauY *big.Int) (*ProvingKey, *VerifyingKey, error) {
	if !isPow2(M) || M == 0 {
		return nil, nil, errors.New("piano: M 须为 2 的幂且 ≥ 1")
	}
	if !isPow2(T) || T == 0 {
		return nil, nil, errors.New("piano: T 须为 2 的幂且 ≥ 1")
	}
	if uint64(len(ci.Ql)) != T {
		return nil, nil, errors.New("piano: 选择子长度与 T 不匹配")
	}

	// ── FFT 域 ────────────────────────────────────────────────────────────────
	domainX := *fft.NewDomain(T)
	domainXL := *fft.NewDomain(4 * T)
	domainY := *fft.NewDomain(M)
	domainYL := *fft.NewDomain(4 * M)

	// ── DKZG SRS ─────────────────────────────────────────────────────────────
	srs, err := dkzg.NewTestSRS(M, T, tauX, tauY)
	if err != nil {
		return nil, nil, err
	}

	// ── 置换多项式 S1、S2、S3（Lagrange 形式）────────────────────────────────
	s1, s2, s3 := computePermutationPolys(ci.Permutation, &domainX)

	// ── 共享多项式的 DKZG 承诺 ───────────────────────────────────────────────
	// 共享多项式（各子节点取值相同）的全局承诺：
	// 对每个子节点 i 调用 CommitLocal(i, poly)，再聚合，
	// 结果等于 g^{poly(τ_X)}（由单位划分性质保证）。
	commitShared := func(poly []fr.Element) (dkzg.Digest, error) {
		localDigests := make([]dkzg.Digest, M)
		for i := uint64(0); i < M; i++ {
			d, err := dkzg.CommitLocal(i, poly, srs)
			if err != nil {
				return dkzg.Digest{}, err
			}
			localDigests[i] = d
		}
		return dkzg.AggregateDigests(localDigests)
	}

	comQl, err := commitShared(ci.Ql)
	if err != nil {
		return nil, nil, err
	}
	comQr, err := commitShared(ci.Qr)
	if err != nil {
		return nil, nil, err
	}
	comQm, err := commitShared(ci.Qm)
	if err != nil {
		return nil, nil, err
	}
	comQo, err := commitShared(ci.Qo)
	if err != nil {
		return nil, nil, err
	}
	comQk, err := commitShared(ci.Qk)
	if err != nil {
		return nil, nil, err
	}
	comS1, err := commitShared(s1)
	if err != nil {
		return nil, nil, err
	}
	comS2, err := commitShared(s2)
	if err != nil {
		return nil, nil, err
	}
	comS3, err := commitShared(s3)
	if err != nil {
		return nil, nil, err
	}

	// ── 组装密钥 ─────────────────────────────────────────────────────────────
	vk := &VerifyingKey{
		SizeX:          T,
		SizeY:          M,
		NbPublicInputs: ci.NbPublicInputs,
		GeneratorX:     domainX.Generator,
		GeneratorY:     domainY.Generator,
		CosetShift:     domainX.FrMultiplicativeGen,
		DKZGSRS:        srs,
		Ql:             comQl,
		Qr:             comQr,
		Qm:             comQm,
		Qo:             comQo,
		Qk:             comQk,
		S1:             comS1,
		S2:             comS2,
		S3:             comS3,
	}

	pk := &ProvingKey{
		Vk:          vk,
		DomainX:     domainX,
		DomainXL:    domainXL,
		DomainY:     domainY,
		DomainYL:    domainYL,
		Ql:          ci.Ql,
		Qr:          ci.Qr,
		Qm:          ci.Qm,
		Qo:          ci.Qo,
		Qk:          ci.Qk,
		S1:          s1,
		S2:          s2,
		S3:          s3,
		Permutation: ci.Permutation,
		DKZGSRS:     srs,
	}

	return pk, vk, nil
}

// ────────────────────────────────────────────────────────────────────────────
// 置换多项式计算
// ────────────────────────────────────────────────────────────────────────────

// computePermutationPolys 从原始置换索引（长度 3T）以 Lagrange 形式返回 S1、S2、S3。
//
//	S1[j] = evaluationIDSmallDomain[permutation[j]]
//	S2[j] = evaluationIDSmallDomain[permutation[T+j]]
//	S3[j] = evaluationIDSmallDomain[permutation[2T+j]]
func computePermutationPolys(permutation []int64, domain *fft.Domain) (s1, s2, s3 []fr.Element) {
	T := int(domain.Cardinality)
	id := getIDSmallDomain(domain)

	s1 = make([]fr.Element, T)
	s2 = make([]fr.Element, T)
	s3 = make([]fr.Element, T)
	for j := 0; j < T; j++ {
		s1[j].Set(&id[permutation[j]])
		s2[j].Set(&id[permutation[T+j]])
		s3[j].Set(&id[permutation[2*T+j]])
	}
	return
}

// ────────────────────────────────────────────────────────────────────────────
// BuildPermutation：从原始 L/R/O 映射表构造 CircuitInfo 所需的置换数组
// ────────────────────────────────────────────────────────────────────────────

// BuildPermutation 从 lro 表（位置 → 变量 ID）构造长度为 3*T 的原始置换数组，
// 将 3T 个连线位置各自映射到对应的变量 ID。
//
//	lro[j]     = 第 j 行 L 线的变量 ID
//	lro[T+j]   = 第 j 行 R 线的变量 ID
//	lro[2T+j]  = 第 j 行 O 线的变量 ID
//
// nbVariables 是不同变量 ID 的总数。
func BuildPermutation(lro []int, nbVariables, T int) []int64 {
	permutation := make([]int64, 3*T)
	for i := range permutation {
		permutation[i] = -1
	}

	// cycle[v] = 变量 v 最近一次出现的位置。
	cycle := make([]int64, nbVariables)
	for i := range cycle {
		cycle[i] = -1
	}

	for i, v := range lro {
		if v < 0 || v >= nbVariables {
			continue
		}
		if cycle[v] != -1 {
			permutation[i] = cycle[v]
		}
		cycle[v] = int64(i)
	}

	// 闭合环：仍为 -1 的位置表示该变量只出现一次，置换目标为其唯一出现位置。
	for i, v := range lro {
		if v < 0 || v >= nbVariables {
			permutation[i] = int64(i) // 自环
			continue
		}
		if permutation[i] == -1 {
			permutation[i] = cycle[v]
		}
	}
	return permutation
}

// ────────────────────────────────────────────────────────────────────────────
// 辅助函数
// ────────────────────────────────────────────────────────────────────────────

// isPow2 判断 n 是否为正的 2 的幂。
func isPow2(n uint64) bool {
	return n > 0 && (n&(n-1)) == 0
}

// dummyFr 仅用于获取 BN254 标量域模数。
type dummyFr struct{ fr.Element }

// Modulus 返回 BN254 标量域的阶。
func (dummyFr) Modulus() *big.Int { return fr.Modulus() }
