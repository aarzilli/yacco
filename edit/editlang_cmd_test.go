package edit

import (
	"testing"
)

func TestSStuck(t *testing.T) {
	testEdit(t, "<uno\ndue\ntre>", "s:^://:", "<//uno\n//due\n//tre>")
}

func TestSEdit(t *testing.T) {
	testEdit(t, "<Humpty Dumpty sat on a wall,\nHumpty Dumpty had a great fall.\nAll the king's horses and all the king's men\nCouldn't put Humpty together again.\n>", `s/\w+/mal/`, "<mal mal mal mal mal mal,\nmal mal mal mal mal mal.\nmal mal mal'mal mal mal mal mal mal'mal mal\nmal'mal mal mal mal mal.\n>")
}

func TestXEdit(t *testing.T) {
	testEdit(t, "<Humpty Dumpty sat on a wall,\nHumpty Dumpty had a great fall.\nAll the king's horses and all the king's men\nCouldn't put Humpty together again.\n>", `x/\w+/ c/mal/`, "<mal mal mal mal mal mal,\nmal mal mal mal mal mal.\nmal mal mal'mal mal mal mal mal mal'mal mal\nmal'mal mal mal mal mal.\n>")

	testEdit(t, "<Humpty Dumpty sat on a wall,\nHumpty Dumpty had a great fall.\nAll the king's horses and all the king's men\nCouldn't put Humpty together again.\n>", `x/\w+/ g/.*al+.*/ c/malkovitch/`, "<Humpty Dumpty sat on a malkovitch,\nHumpty Dumpty had a great malkovitch.\nAll the king's horses and malkovitch the king's men\nCouldn't put Humpty together again.\n>")

	testEdit(t, "<Humpty Dumpty sat on a wall,\nHumpty Dumpty had a great fall.\nAll the king's horses and all the king's men\nCouldn't put Humpty together again.\n>", `x/\n/ c/malkovitch/`, "<Humpty Dumpty sat on a wall,malkovitchHumpty Dumpty had a great fall.malkovitchAll the king's horses and all the king's menmalkovitchCouldn't put Humpty together again.>malkovitch")

}

func TestYEdit(t *testing.T) {
	testEdit(t, "<Humpty Dumpty sat on a wall,\nHumpty Dumpty had a great fall.\nAll the king's horses and all the king's men\nCouldn't put Humpty together again.\n>", `y/\n/ c/malkovitch/`, "<malkovitch\nmalkovitch\nmalkovitch\nmalkovitch\n>")
}

func TestSExtraBolBug(t *testing.T) {
	testEdit(t, "zero\n<1\n2\n3\n>extra\n", `s/^/!/`, "zero\n<!1\n!2\n!3\n>extra\n")
}

func TestSEOLAppend(t *testing.T) {
	testEdit(t, "zero\n<1\n2\n3\n>extra\n", `s/$/!/`, "zero\n<1!\n2!\n3!\n>extra\n")
}

func TestGBug(t *testing.T) {
	testEdit(t, "<bip i bang iii baip i bop>", `x/\w+/g/i/c/na/`, "<bip na bang iii baip na bop>")
}

func TestSWithBackrefEdit(t *testing.T) {
	testEdit(t, "<01 12 23 34 45 56 67 78 89 9A AB BC CD DE EF\n>", `s/(\S\S)/0x\1/`, "<0x01 0x12 0x23 0x34 0x45 0x56 0x67 0x78 0x89 0x9A 0xAB 0xBC 0xCD 0xDE 0xEF\n>")
}

func TestXWithIEdit(t *testing.T) {
	testEdit(t, "<01 12 23 34 45 56 67 78 89 9A AB BC CD DE EF\n>", `x/\S\S/i/0x/`, "<0x01 0x12 0x23 0x34 0x45 0x56 0x67 0x78 0x89 0x9A 0xAB 0xBC 0xCD 0xDE 0xEF\n>")
	testEdit(t, "<01 12 23 34 45 56 67 78 89 9A AB BC CD DE EF\n>", `x/\S\S/a/,/`, "<01, 12, 23, 34, 45, 56, 67, 78, 89, 9A, AB, BC, CD, DE, EF,\n>")
}

func TestXOmit(t *testing.T) {
	testEdit(t, "<bip i bang iii baip i bop>", `xg/i/c/na/`, "<bip na bang iii baip na bop>")
}
