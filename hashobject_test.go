package main

import (
	"strings"
	"testing"
)

func TestHashObject(t *testing.T) {
	tests := []struct {
		ObjType string
		Data    string
		Hash    string
	}{
		// These are all hashes compared against the official git client.
		{"blob", "test\n", "9daeafb9864cf43055ae93beb0afd6c7d144bfa4"},
		{
			"commit",
			`tree adbfd4aadd70c1b26fcfff59b085045786d3b7c0
parent 20648e724aaed71fcdc88aa806ee2f8ebe3fed07
author Dave MacFarlane <driusan@gmail.com> 1481944775 -0500
committer Dave MacFarlane <driusan@gmail.com> 1481944775 -0500

Fixed bug where git push wasn't working since refactoring Sha1 to own type

`,
			"1e55454eeac0eb54e22fe5edac213cedcd7a9acb",
		},
		{
			// I can't get any variation of git ls-tree | git hash-object -t tree --stdin
			// to not complain about an invalid tree for any tree but the empty case, but
			// at least this makes sure the empty case is handled properly.
			"tree", "", "4b825dc642cb6eb9a060e54bf8d69288fbee4904",
		},
		{
			// This is compared against git hash-object --stdin --literally on the real git
			// client. HashReader should probably be returning an error.

			"tree",
			`100644 blob 6061341c75bce327d64e8847d8eae02b725276ac	.gitignore
100644 blob cf5fbd5922f090981b28dbc50f61e18d2ca61898	README.md
100644 blob 40de5fe3762d5d3a5c8d7809960d765f840aefc7	add.go
100644 blob 8024e84b9f9e8129d82deba4db15227bb88899e8	branch.go
100644 blob 32f1df1cf1ccbc4f14862826461cf4194827b0f8	checkout.go
100644 blob bde64e462b92ea3dab8e9856b41f933c51046cf7	client.go
100644 blob f0067c4d1f839edbcd31ebaa16c043eebc73138f	client_hacks.go
100644 blob 540e8a2cd6af0eff55571d9213ab4bfc2c3cc0e7	clone.go
100644 blob 407518d25ac50e7795e6e6727a57cc76859a2ed1	commit.go
100644 blob d641deca723b745b38476226aa7e7deaf89baee3	committree.go
100644 blob 1b4a9b7e1dc937bf0294eb9fbaaa87bfb24885c8	config.go
100644 blob c81328b8138e998815ddac34b9b0246cd0714304	fetch.go
100644 blob f1f1fc23108b52322ec4907fb207c3f9d98e5f07	hashobject.go
100644 blob 74ef9eac5a3376ca015632f0bbe2ca929c91347c	index.go
100644 blob ecc61a860f471419d305168bb09a65c8b2ba39e2	init.go
040000 tree ae7bb05c3d91d81238923a485f1d8c0d94cbc0e5	issues
100644 blob 85a28dc9c36670624ed6003f73cf4a57e42ed03a	log.go
100644 blob 38e6034f8c7abf7715d4e535b351338bcbab9e45	lstree.go
100644 blob 5193c7552162709ce35e80130e2cd225b0498ee2	main.go
100644 blob 12485d80c362aae5ba93da18a98183e132471c90	merge.go
100644 blob 00b032ce1452f4258b264b613f4c292e2356f3ac	mergebase.go
100644 blob 0e7e7f67836d45ca3570820fada182dfeb6b51a9	objects.go
100644 blob 0518dfaa86598e7f177f51f80f6192dd5dd5cb2b	packfile.go
100644 blob 50e89f07f0070a7d74932b64a6b7b5b51732ec40	packobjects.go
100644 blob 0c94a8b7586c454c23e0a67f4d1301711a0c6ed3	pktline_test.go
100644 blob e9611bac0663ccc6f6caab27ff675c49ce7f29f4	push.go
100644 blob 15d744be7888cb21b379e0ab72affef13a3690b6	reset.go
100644 blob a8bd5582e5bdd1bc8ed009b4a5deedda85334cea	retrieve.go
100644 blob b77269b6629f7c7b0699bd56f33d30c18726030b	rev-list.go
100644 blob a583758440e82cd026c760e022937ed087ed3f52	rev-parse.go
100644 blob 523ed3692a89862a30caac0aa6b6b16227245b86	send-pack.go
100644 blob 9cdab927cc29610ca127477be18402fe971ab134	sha1.go
100644 blob 38234ef152ec1bb522f0cb545e1399b1da22742f	status.go
100644 blob 0b5f68617aca15fe812c37b9211852d9fd87b7de	status.txt
100644 blob 805bb582f1701ba89d568a65dcf27ad98baa6a1f	symbolicref.go
100644 blob 983c5201e5773f8b7035c57a0300ff6ee6292faf	updateref.go
100644 blob 3503a1b11be84da1d5508c5312920fb7d3d9b2a8	writetree.go
040000 tree d074f72be9045a5e63800698ac373ca7a4b491b3	zlib
`,
			"8b84053ce1240e74abb731ad85f4e2eacede3b16",
		},
	}

	for i, tc := range tests {
		sha1, data, err := HashReader(tc.ObjType, strings.NewReader(tc.Data))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != tc.Data {
			t.Fatalf("Returnd data does not match passed data for test %d.", i)
		}
		if sha1.String() != tc.Hash {
			t.Errorf("Unexpected hash for %d: got %v want %v", i, sha1.String(), tc.Hash)
		}
	}
}
