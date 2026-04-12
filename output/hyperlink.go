package output

// Hyperlink returns an OSC-8 terminal hyperlink when stdout is interactive,
// or the plain URL otherwise. If label is empty, the URL is used as the label.
func Hyperlink(url, label string) string {
	return hyperlinkFor(IsInteractive(), url, label)
}

func hyperlinkFor(interactive bool, url, label string) string {
	if url == "" {
		return ""
	}
	if label == "" {
		label = url
	}
	if !interactive {
		return url
	}
	return "\x1b]8;;" + url + "\x1b\\" + label + "\x1b]8;;\x1b\\"
}
