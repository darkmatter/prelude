package shared

import (
	"io"

	"github.com/charmbracelet/colorprofile"
)

// ColorWriter wraps an io.Writer with a colorprofile.Writer that downgrades
// colors for non-truecolor terminals. Both menu-tui and motd use this to
// preserve terminal color while stripping ANSI when output is piped.
func ColorWriter(output io.Writer, environ []string, profileName string) *colorprofile.Writer {
	return ColorWriterFor(output, output, environ, profileName)
}

// ColorWriterFor is ColorWriter for bytes bound for another destination: the
// profile is detected against terminal while writes go to output. Library
// hosts (libprelude) render into a buffer that their caller prints to the
// process's own stdout, so detection must look at that stdout, not the buffer.
func ColorWriterFor(output, terminal io.Writer, environ []string, profileName string) *colorprofile.Writer {
	w := &colorprofile.Writer{Forward: output, Profile: colorprofile.Detect(terminal, environ)}
	if profile, ok := ConfiguredColorProfile(profileName); ok && w.Profile != colorprofile.NoTTY {
		w.Profile = profile
	}
	return w
}

// ConfiguredColorProfile translates a config string ("truecolor", "ansi256")
// to a colorprofile.Profile. Returns false for "auto"/empty so the writer
// uses its detected terminal profile.
func ConfiguredColorProfile(name string) (colorprofile.Profile, bool) {
	switch name {
	case "truecolor":
		return colorprofile.TrueColor, true
	case "ansi256":
		return colorprofile.ANSI256, true
	default:
		return colorprofile.Unknown, false
	}
}
