package git

import (
	"testing"
)

func TestSplitPatch(t *testing.T) {
	tests := []struct {
		patch    string
		expected []patchHunk
	}{
		// A simple patch with one hunk, it should just extract
		// the filename
		{
			`diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,5 +1,5 @@
 package main
-
+// This is a test
 import (
 	"errors"
 	"flag"
`,

			[]patchHunk{
				{
					"main.go",
					`@@ -1,5 +1,5 @@
 package main
-
+// This is a test
 import (
 	"errors"
 	"flag"
`,
				},
			},
		},
		// The same patch with the extra overhead that format-patch
		// adds
		{
			`From e09034343434 Mon fooo
From: Foo <root@bar.com>
Date: Sat, 6 Jan 2018 16:18:19 -0500
Subject: [Patch] Bar

--
 foo.txt | 1 +
 1 file changed, 1 insertion(+)

diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,5 +1,5 @@
 package main
-
+// This is a test
 import (
 	"errors"
 	"flag"
--
2.14.2
`,
			[]patchHunk{
				{
					"main.go",
					`@@ -1,5 +1,5 @@
 package main
-
+// This is a test
 import (
 	"errors"
 	"flag"
--
2.14.2
`,
				},
			},
		},
		// A test with multiple files, multiple hunks within one of them,
		// and a file in a subdirectory.
		// This should be pretty comprehensive.
		{
			`
diff --git a/git/apply.go b/git/apply.go
--- a/git/apply.go
+++ b/git/apply.go
@@ -34,7 +34,7 @@
 	NoAdd bool
 
 	ExcludePattern, IncludePattern string
-
+	// Test
 	InaccurateEof bool
 
 	Verbose bool

diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -19,7 +19,7 @@
 		return true
 	}
 }
-
+// doooo
 var subcommand, subcommandUsage string
 
 func main() {
@@ -244,3 +244,5 @@
 		fmt.Fprintf(os.Stderr, "Unknown git command %s.\n", subcommand)
 	}
 }
+
+//Test
`,
			[]patchHunk{
				{
					"git/apply.go",
					`@@ -34,7 +34,7 @@
 	NoAdd bool
 
 	ExcludePattern, IncludePattern string
-
+	// Test
 	InaccurateEof bool
 
 	Verbose bool

`},
				{
					"main.go",
					`@@ -19,7 +19,7 @@
 		return true
 	}
 }
-
+// doooo
 var subcommand, subcommandUsage string
 
 func main() {
`,
				},
				{
					"main.go",
					`@@ -244,3 +244,5 @@
 		fmt.Fprintf(os.Stderr, "Unknown git command %s.\n", subcommand)
 	}
 }
+
+//Test
`,
				},
			},
		},
		// A patch where the divider doesn't have the commas
		// (This is a case which came up when trying to implement dgit checkout -p)
		{
			`diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -2 +2 @@
-
+//This is a test22
@@ -234,0 +235,5 @@
+		fmt.Fprintln(os.Stderr, err)
+		os.Exit(4)
+	}
+	case "apply":
+		if err := cmd.Apply(c, args; err != nil {
`,
			[]patchHunk{
				{
					"main.go",
					`@@ -2 +2 @@
-
+//This is a test22
`,
				},
				{
					"main.go",
					`@@ -234,0 +235,5 @@
+		fmt.Fprintln(os.Stderr, err)
+		os.Exit(4)
+	}
+	case "apply":
+		if err := cmd.Apply(c, args; err != nil {
`,
				},
			},
		},
	}

	for i, tc := range tests {
		val, err := splitPatch(tc.patch, false)
		if err != nil {
			t.Errorf("Case %d: %v", i, err)
			continue
		}
		if len(val) != len(tc.expected) {
			t.Errorf("Case %d: got %v want %v", i, val, tc.expected)
			continue
		}
		for j := range val {
			if val[j] != tc.expected[j] {
				t.Errorf("Case %d: got %v want %v", i, val, tc.expected)
				continue
			}
		}
	}
}
