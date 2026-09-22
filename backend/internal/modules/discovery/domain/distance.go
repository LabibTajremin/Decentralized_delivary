package domain

import "strconv"

// bengaliDigits maps an ASCII digit to its Bengali form.
//
// A table rather than an offset calculation because it makes the mapping
// readable by anyone auditing the output, and because it is used on strings a
// customer reads.
var bengaliDigits = [10]string{"০", "১", "২", "৩", "৪", "৫", "৬", "৭", "৮", "৯"}

// FormatDistance renders a distance the way a customer should read it.
//
// The server composes it, like every other display string (2.9): a client
// rounding metres itself will round differently from the receipt, and the
// customer comparing "1.2 km" on one screen with "1 km" on the next has no way
// to know which is right. Bengali-first, because the audience is (1.4).
//
// Below a kilometre the answer is in metres rounded to fifty, because a shop
// "৩৫০ মিটার" away is a useful fact and "৩৪৭ মিটার" is false precision over a
// straight-line distance nobody walks. Above it, one decimal place.
func FormatDistance(metres float64, lang string) string {
	bengali := lang != "en"
	if metres < 0 {
		metres = 0
	}

	if metres < 1000 {
		rounded := int(metres/50+0.5) * 50
		if rounded == 0 {
			// Anything under 25 m is "at your location" rather than "0 m",
			// which reads like a bug.
			if bengali {
				return "খুব কাছেই"
			}
			return "Very close"
		}
		if bengali {
			return toBengaliDigits(strconv.Itoa(rounded)) + " মিটার"
		}
		return strconv.Itoa(rounded) + " m"
	}

	km := strconv.FormatFloat(metres/1000, 'f', 1, 64)
	if bengali {
		return toBengaliDigits(km) + " কিমি"
	}
	return km + " km"
}

// toBengaliDigits rewrites the ASCII digits in s, leaving everything else —
// the decimal point especially — as it is.
func toBengaliDigits(s string) string {
	out := make([]byte, 0, len(s)*3)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			out = append(out, bengaliDigits[c-'0']...)
			continue
		}
		out = append(out, c)
	}
	return string(out)
}

// FormatRadius renders a search radius for the expansion prompt.
//
// Always kilometres, with no decimal when it is whole: radii come from
// configuration in whole kilometres, and "৫ কিমি" is what an operator set.
func FormatRadius(metres float64, lang string) string {
	bengali := lang != "en"
	km := metres / 1000
	text := strconv.FormatFloat(km, 'f', -1, 64)
	if bengali {
		return toBengaliDigits(text) + " কিমি"
	}
	return text + " km"
}
