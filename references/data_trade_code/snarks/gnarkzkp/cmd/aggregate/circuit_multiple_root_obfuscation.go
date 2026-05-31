package main

import "github.com/consensys/gnark/frontend"

type MultipleRootObfuscationProof struct {
	Leaf  [10]frontend.Variable `gnark:",public"`
	Path  [10][]frontend.Variable
	Root0 frontend.Variable `gnark:",public"`
	Root1 frontend.Variable `gnark:",public"`
	Root2 frontend.Variable `gnark:",public"`
	Root3 frontend.Variable `gnark:",public"`

	Index0 frontend.Variable
	Index1 frontend.Variable
}

func checkRootObfuscation(api frontend.API, mimchash mimc.MiMC, leaf frontend.Variable, path []frontend.Variable, root frontend.Variable) {
	mimchash.Reset()
	for _, path := range path {
		mimchash.Write(path)
	}
	api.AssertIsEqual(mimchash.Sum(), root)
}

func (rop *MultipleRootObfuscationProof) Define(api frontend.API) error {
	h, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}
	var sum frontend.Variable
	node := rop.Leaf[0]
	for len(rop.Path[0]) != 0 {
		h.Reset()
		brotherHash := rop.Path[0][0]
		rop.Path[0] = rop.Path[0][1:]
		h.Write(node)
		h.Write(brotherHash)
		sum = h.Sum()
		node = sum
	}
	api.Println("sum: ", sum)
	root := api.Lookup2(rop.Index0, rop.Index1, rop.Root0, rop.Root1, rop.Root2, rop.Root3)
	api.AssertIsEqual(sum, root)
	return nil
}