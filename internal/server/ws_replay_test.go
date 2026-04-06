package server

import "testing"

func TestAlignReplayToLineStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "already line start",
			in:   "\n$ ls\nfile\n",
			want: "\n$ ls\nfile\n",
		},
		{
			name: "trim partial prefix at newline",
			in:   "11;rgb:aaaa/bbbb/cccc\n/workspace $ ls\n",
			want: "/workspace $ ls\n",
		},
		{
			name: "trim partial prefix at carriage return",
			in:   "garbage prefix\rprompt$ ",
			want: "prompt$ ",
		},
		{
			name: "no line boundary drops replay",
			in:   "partial_escape_without_newline",
			want: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := alignReplayToLineStart([]byte(tt.in))
			if string(got) != tt.want {
				t.Fatalf("alignReplayToLineStart() = %q, want %q", string(got), tt.want)
			}
		})
	}
}
