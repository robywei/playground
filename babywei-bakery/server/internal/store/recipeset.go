package store

import "fmt"

// RecipeSet 是配方查表，key 為 ID。
//
// 定義在 store 而非 domain，是為了避免循環依賴：domain 已經 import store，
// 所以 store 不能 import domain。domain 以 type alias 沿用本型別。
type RecipeSet struct {
	Doughs   map[string]Dough   `json:"doughs"`
	Fillings map[string]Filling `json:"fillings"`
}

// RecipeSet 一次載入全部配方，供 domain 的計算函數使用。
func (d *DB) RecipeSet() (RecipeSet, error) {
	doughs, err := d.ListDoughs()
	if err != nil {
		return RecipeSet{}, fmt.Errorf("載入產品配方: %w", err)
	}
	fillings, err := d.ListFillings()
	if err != nil {
		return RecipeSet{}, fmt.Errorf("載入配料: %w", err)
	}
	set := RecipeSet{
		Doughs:   make(map[string]Dough, len(doughs)),
		Fillings: make(map[string]Filling, len(fillings)),
	}
	for _, g := range doughs {
		set.Doughs[g.ID] = g
	}
	for _, f := range fillings {
		set.Fillings[f.ID] = f
	}
	return set, nil
}
