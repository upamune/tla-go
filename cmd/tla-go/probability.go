package main

import (
	"math"
	"strconv"
	"strings"
)

// formatProbability follows TLC's two-significant-digit ProbabilityToString.
func formatProbability(value float64) string {
	if value == 0 {
		return "0.0"
	}
	text := javaFloat(value)
	cursor := probabilityCursor{text: text}
	cursor.integer()
	if cursor.position == len(text) {
		return cursor.result
	}
	if text[cursor.position] != '.' {
		return text
	}
	cursor.fraction()
	if cursor.position == len(text) {
		return cursor.result
	}
	if text[cursor.position] == 'E' {
		return cursor.result + text[cursor.position:]
	}
	return text
}

type probabilityCursor struct {
	text, result     string
	position, digits int
}

func (c *probabilityCursor) integer() {
	for c.position < len(c.text) && c.text[c.position] == '0' {
		c.position++
	}
	for c.isDigit() {
		c.result += c.text[c.position : c.position+1]
		c.position++
		c.digits++
	}
}

func (c *probabilityCursor) fraction() {
	c.position++
	if c.digits >= 2 {
		c.skipDigits()
		return
	}
	c.result += "."
	if c.digits == 0 {
		for c.position < len(c.text) && c.text[c.position] == '0' {
			c.result += "0"
			c.position++
		}
	}
	c.significantFraction()
	c.skipDigits()
}

func (c *probabilityCursor) significantFraction() {
	for c.isDigit() && c.digits < 2 {
		c.result += c.text[c.position : c.position+1]
		c.position++
		c.digits++
	}
	if c.isDigit() && c.text[c.position] >= '5' {
		c.result = roundProbabilityDigits(c.result)
	}
}

func (c *probabilityCursor) isDigit() bool {
	return c.position < len(c.text) && c.text[c.position] >= '0' && c.text[c.position] <= '9'
}

func (c *probabilityCursor) skipDigits() {
	for c.isDigit() {
		c.position++
	}
}

func roundProbabilityDigits(digits string) string {
	if digits == "" {
		return "1"
	}
	last := digits[len(digits)-1]
	front := digits[:len(digits)-1]
	if last == '.' {
		return roundProbabilityDigits(front) + "."
	}
	if last == '9' {
		return roundProbabilityDigits(front) + "0"
	}
	return front + string(last+1)
}

func javaFloat(value float64) string {
	if magnitude := math.Abs(value); magnitude >= 1e-3 && magnitude < 1e7 {
		text := strconv.FormatFloat(value, 'f', -1, 64)
		if !strings.Contains(text, ".") {
			text += ".0"
		}
		return text
	}
	parts := strings.Split(strconv.FormatFloat(value, 'e', -1, 64), "e")
	if !strings.Contains(parts[0], ".") {
		parts[0] += ".0"
	}
	exponent, _ := strconv.Atoi(parts[1])
	return parts[0] + "E" + strconv.Itoa(exponent)
}
