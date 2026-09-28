package newapipricing

import (
	"math/big"
	"strings"

	"kwu/internal/modelbilling"
)

// exactDecimal is intentionally limited to the finite decimal operations used
// by the documented pricing formulas.
type exactDecimal struct {
	coefficient *big.Int
	scale       int
}

func parseExactDecimal(value modelbilling.DecimalString) exactDecimal {
	raw := string(value)
	integer, fraction, found := strings.Cut(raw, ".")
	digits := integer
	scale := 0
	if found {
		digits += fraction
		scale = len(fraction)
	}
	coefficient, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		panic("validated DecimalString could not be parsed")
	}
	return exactDecimal{coefficient: coefficient, scale: scale}
}

func zeroExactDecimal() exactDecimal {
	return exactDecimal{coefficient: new(big.Int), scale: 0}
}

func (d exactDecimal) add(other exactDecimal) exactDecimal {
	scale := max(d.scale, other.scale)
	left := new(big.Int).Set(d.coefficient)
	right := new(big.Int).Set(other.coefficient)
	if d.scale < scale {
		left.Mul(left, decimalPower(scale-d.scale))
	}
	if other.scale < scale {
		right.Mul(right, decimalPower(scale-other.scale))
	}
	return exactDecimal{coefficient: left.Add(left, right), scale: scale}
}

func (d exactDecimal) multiply(other exactDecimal) exactDecimal {
	return exactDecimal{
		coefficient: new(big.Int).Mul(d.coefficient, other.coefficient),
		scale:       d.scale + other.scale,
	}
}

func (d exactDecimal) multiplyInt64(value int64) exactDecimal {
	return exactDecimal{coefficient: new(big.Int).Mul(d.coefficient, big.NewInt(value)), scale: d.scale}
}

func (d exactDecimal) divideMillion() exactDecimal {
	d.scale += 6
	return d
}

func (d exactDecimal) decimalString() modelbilling.DecimalString {
	coefficient := new(big.Int).Set(d.coefficient)
	scale := d.scale
	if coefficient.Sign() == 0 {
		return "0"
	}
	ten := big.NewInt(10)
	for scale > 0 {
		quotient, remainder := new(big.Int).QuoRem(coefficient, ten, new(big.Int))
		if remainder.Sign() != 0 {
			break
		}
		coefficient = quotient
		scale--
	}
	digits := coefficient.String()
	if scale == 0 {
		return modelbilling.DecimalString(digits)
	}
	if len(digits) <= scale {
		return modelbilling.DecimalString("0." + strings.Repeat("0", scale-len(digits)) + digits)
	}
	point := len(digits) - scale
	return modelbilling.DecimalString(digits[:point] + "." + digits[point:])
}

func decimalPower(power int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(power)), nil)
}
