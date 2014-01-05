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

	testEdit(t, "<Humpty Dumpty sat on a wall,\nHumpty Dumpty had a great fall.\nAll the king's horses and all the king's men\nCouldn't put Humpty together again.\n>", `x/\w+/ g/al+/ c/malkovitch/`, "<Humpty Dumpty sat on a malkovitch,\nHumpty Dumpty had a great malkovitch.\nAll the king's horses and malkovitch the king's men\nCouldn't put Humpty together again.\n>")

	testEdit(t, "<Humpty Dumpty sat on a wall,\nHumpty Dumpty had a great fall.\nAll the king's horses and all the king's men\nCouldn't put Humpty together again.\n>", `x/\n/ c/malkovitch/`, "<Humpty Dumpty sat on a wall,malkovitchHumpty Dumpty had a great fall.malkovitchAll the king's horses and all the king's menmalkovitchCouldn't put Humpty together again.>malkovitch")

}

func TestYEdit(t *testing.T) {
	testEdit(t, "<Humpty Dumpty sat on a wall,\nHumpty Dumpty had a great fall.\nAll the king's horses and all the king's men\nCouldn't put Humpty together again.\n>", `y/\n/ c/malkovitch/`, "<malkovitch\nmalkovitch\nmalkovitch\nmalkovitch\n>")
}

func TestSExtraBolBug(t *testing.T) {
	testEdit(t, "zero\n<1\n2\n3\n>extra\n", `s/^/!/`, "zero\n<!1\n!2\n!3\n>extra\n")
}
