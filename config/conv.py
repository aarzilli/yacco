#!/usr/bin/env python

def convert(path, name):
	v = open(path).read()

	out = open("config/" + name + ".go", "w")
	out.write("package config\n")
	out.write("var " + name + " = []byte{\n\t")
	for c in v:
		out.write(str(ord(c)) + ", ")
	out.write("}\n")
	out.close()

convert("config/luxisr.ttf", "luxibytes")
convert("config/luximr.ttf", "luximonobytes")
