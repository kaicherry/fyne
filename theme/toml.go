package theme

import (
	"encoding/hex"
	"errors"
	"image/color"

	"fyne.io/fyne/v2"
	"github.com/BurntSushi/toml"
)

// FromTOML returns a Theme created from the given TOML metadata.
// Any values not present in the data will fall back to the default theme.
//
// Since: 2.2
func FromTOML(data string) fyne.Theme {
	var th *schema
	if _, err := toml.Decode(data, &th); err != nil {
		fyne.LogError("ERR", err)
		return DefaultTheme()
	}

	return &tomlTheme{data: th, fallback: DefaultTheme()}
}

type hexColor string

func (h hexColor) color() (color.Color, error) {
	data := h
	switch len(h) {
	case 8, 6:
	case 9, 7: // remove # prefix
		data = h[1:]
	case 5: // remove # prefix, then double up
		data = h[1:]
		fallthrough
	case 4: // could be rgba or #rgb
		if data[0] == '#' {
			v := []rune(data[1:])
			data = hexColor([]rune{v[0], v[0], v[1], v[1], v[2], v[2]})
			break
		}

		v := []rune(data)
		data = hexColor([]rune{v[0], v[0], v[1], v[1], v[2], v[2], v[3], v[3]})
	case 3:
		v := []rune(h)
		data = hexColor([]rune{v[0], v[0], v[1], v[1], v[2], v[2]})
	default:
		return color.Transparent, errors.New("invalid color format: " + string(h))
	}

	digits, err := hex.DecodeString(string(data))
	if err != nil {
		return nil, err
	}
	ret := &color.NRGBA{R: digits[0], G: digits[1], B: digits[2]}
	if len(digits) == 4 {
		ret.A = digits[3]
	} else {
		ret.A = 0xff
	}

	return ret, nil
}

type schema struct {
	Colors      map[string]hexColor `toml:",omitempty"`
	DarkColors  map[string]hexColor `toml:"Colors-dark,omitempty"`
	LightColors map[string]hexColor `toml:"Colors-light,omitempty"`
	Sizes       map[string]float32  `toml:",omitempty"`
}

type tomlTheme struct {
	data     *schema
	fallback fyne.Theme
}

func (t *tomlTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch variant {
	case VariantLight:
		if val, ok := t.data.LightColors[string(name)]; ok {
			c, err := val.color()
			if err != nil {
				fyne.LogError("Failed to parse color", err)
			} else {
				return c
			}
		}
	case VariantDark:
		if val, ok := t.data.DarkColors[string(name)]; ok {
			c, err := val.color()
			if err != nil {
				fyne.LogError("Failed to parse color", err)
			} else {
				return c
			}
		}
	}

	if val, ok := t.data.Colors[string(name)]; ok {
		c, err := val.color()
		if err != nil {
			fyne.LogError("Failed to parse color", err)
		} else {
			return c
		}
	}

	return t.fallback.Color(name, variant)
}

func (t *tomlTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.fallback.Font(style)
}

func (t *tomlTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.fallback.Icon(name)
}

func (t *tomlTheme) Size(name fyne.ThemeSizeName) float32 {
	if val, ok := t.data.Sizes[string(name)]; ok {
		return val
	}

	return t.fallback.Size(name)
}
