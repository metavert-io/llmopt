package main

import "strings"

// The brand form edits list fields (products, categories, use cases,
// differentiators) as comma-separated text. It used to split at every comma,
// so "Starter Kit (monthly plan, 50 seats)" was saved as two items, "Starter
// Kit (monthly plan" and "50 seats)". The form now keeps commas inside brackets; these
// helpers repair lists that were already split.

// bracketBalance counts opening minus closing round and square brackets.
func bracketBalance(s string) int {
	return strings.Count(s, "(") + strings.Count(s, "[") - strings.Count(s, ")") - strings.Count(s, "]")
}

// rejoinSplitItems joins an item that opens a bracket with the items after it,
// up to the one that closes it. An item whose bracket is never closed is left
// alone rather than swallowing the rest of the list.
func rejoinSplitItems(items []string) []string {
	out := make([]string, 0, len(items))
	for i := 0; i < len(items); i++ {
		bal := bracketBalance(items[i])
		if bal <= 0 {
			out = append(out, items[i])
			continue
		}
		end := -1
		for j := i + 1; j < len(items); j++ {
			bal += bracketBalance(items[j])
			if bal <= 0 {
				end = j
				break
			}
		}
		if end < 0 {
			out = append(out, items[i])
			continue
		}
		out = append(out, strings.Join(items[i:end+1], ", "))
		i = end
	}
	return out
}

func repairBrandLists(b *BrandProfile) {
	b.Categories = rejoinSplitItems(b.Categories)
	b.Products = rejoinSplitItems(b.Products)
	b.KeyUseCases = rejoinSplitItems(b.KeyUseCases)
	b.Differentiators = rejoinSplitItems(b.Differentiators)
}
