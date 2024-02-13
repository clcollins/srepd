package tui

import "testing"

//lint:ignore U1000 - future proofing
func TestUpdateIncidentList(t *testing.T) {
	// updateIncidentList accepts a pointer to a pd.Config struct
	// and returns a tea.Cmd function. The tea.Cmd function returns
	// a tea.Msg function. The tea.Msg function returns an
	// updatedIncidentListMsg struct and an error.

}

func TestStringSlicer(t *testing.T) {
	// stringSlicer accepts a string and returns a string slice
	// containing the characters of the string.

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single word string",
			input:    "xterm",
			expected: []string{"xterm"},
		},
		{
			name:     "multi word string",
			input:    "flatpak run org.contourterminal.Contour",
			expected: []string{"flatpak", "run", "org.contourterminal.Contour"},
		},
		{
			name:     "multi word string with dashes and slashes",
			input:    "/usr/local/bin/gnome-terminal",
			expected: []string{"/usr/local/bin/gnome-terminal"},
		},
		{
			name:     "multi word string with dashes and slashes and arguments",
			input:    "/usr/local/bin/gnome-terminal --load-config=~/gnome-comfig --no-environment",
			expected: []string{"/usr/local/bin/gnome-terminal", "--load-config=~/gnome-comfig", "--no-environment"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := stringSlicer(tt.input)
			for i := range actual {
				if actual[i] != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected[i], actual[i])
				}
			}
		})
	}
}
