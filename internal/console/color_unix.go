// +build darwin dragonfly freebsd linux netbsd openbsd

package console

import (
	"fmt"
)

var foregroundSequences, backgroundSequences map[ColorValue]string

func init() {
	foregroundSequences = map[ColorValue]string{
		BLACK:        "\033[0;30m",
		BLUE:         "\033[0;34m",
		GREEN:        "\033[0;32m",
		CYAN:         "\033[0;36m",
		RED:          "\033[0;31m",
		MAGENTA:      "\033[0;35m",
		BROWN:        "\033[0;33m",
		LIGHTGREY:    "\033[0;37m",
		DARKGREY:     "\033[1;30m",
		LIGHTBLUE:    "\033[1;34m",
		LIGHTGREEN:   "\033[1;32m",
		LIGHTCYAN:    "\033[1;36m",
		LIGHTRED:     "\033[1;31m",
		LIGHTMAGENTA: "\033[1;35m",
		YELLOW:       "\033[1;33m",
		WHITE:        "\033[1;37m",
	}

	backgroundSequences = map[ColorValue]string{
		BLACK:        "\033[40m",
		BLUE:         "\033[44m",
		GREEN:        "\033[42m",
		CYAN:         "\033[46m",
		RED:          "\033[41m",
		MAGENTA:      "\033[45m",
		BROWN:        "\033[43m",
		LIGHTGREY:    "\033[47m",
		DARKGREY:     "\033[40m",
		LIGHTBLUE:    "\033[44m",
		LIGHTGREEN:   "\033[42m",
		LIGHTCYAN:    "\033[46m",
		LIGHTRED:     "\033[41m",
		LIGHTMAGENTA: "\033[45m",
		YELLOW:       "\033[43m",
		WHITE:        "\033[47m",
	}
}

func SetTextColor(color ColorValue) {
	fmt.Print(foregroundSequences[color])
}

func SetBackColor(color ColorValue) {
	fmt.Print(backgroundSequences[color])
}

func SetColor(foreground, background ColorValue) {
	fmt.Print(foregroundSequences[foreground])
	fmt.Print(backgroundSequences[background])
}

func ResetColor() {
	fmt.Print("\033[0m")
}
