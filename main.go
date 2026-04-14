package main

func main() {
	ch := '2'
	println(isLetter(byte(ch)))
}

func isLetter(ch byte) bool {
	return ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') || (ch == '_')
}
