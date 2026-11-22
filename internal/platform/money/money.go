// Package money holds the single deterministic money-allocation algorithm used
// by Pricing and AfterSales. ADR 0005 §6.2 forbids a second rounding
// implementation anywhere else in the repository.
package money

import "math/big"

// AllocateProportional splits total across weights with the largest-remainder
// method. The result is deterministic, always sums to total, and the last
// element absorbs any residual so callers can verify conservation with a plain
// sum. Negative weights are treated as zero; a positive total with no positive
// weight is assigned to the last allocator (never lost).
func AllocateProportional(total int64, weights []int64) []int64 {
	out := make([]int64, len(weights))
	if len(weights) == 0 || total <= 0 {
		return out
	}
	sum := new(big.Int)
	for _, weight := range weights {
		if weight > 0 {
			sum.Add(sum, big.NewInt(weight))
		}
	}
	if sum.Sign() == 0 {
		out[len(out)-1] = total
		return out
	}
	totalBig := big.NewInt(total)
	remainders := make([]*big.Int, len(weights))
	allocated := int64(0)
	for i, weight := range weights {
		if weight <= 0 {
			remainders[i] = new(big.Int)
			continue
		}
		numerator := new(big.Int).Mul(totalBig, big.NewInt(weight))
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(numerator, sum, remainder)
		out[i] = quotient.Int64()
		allocated += out[i]
		remainders[i] = remainder
	}
	left := total - allocated
	if left > 0 {
		// Stable selection: larger remainder first, then lower index. Selection
		// sort keeps the result byte-for-byte stable across runs.
		order := make([]int, 0, len(weights))
		for i := range weights {
			if weights[i] > 0 {
				order = append(order, i)
			}
		}
		for i := 1; i < len(order); i++ {
			for j := i; j > 0; j-- {
				a, b := order[j-1], order[j]
				cmp := remainders[a].Cmp(remainders[b])
				if cmp > 0 || (cmp == 0 && a < b) {
					break
				}
				order[j-1], order[j] = order[j], order[j-1]
			}
		}
		for _, index := range order {
			if left == 0 {
				break
			}
			out[index]++
			left--
		}
	}
	// Defensive residual absorption. With the loop above this is unreachable,
	// but keeping it makes the conservation invariant unconditional.
	if left != 0 {
		out[len(out)-1] += left
	}
	return out
}
