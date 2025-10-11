package test

import (
	"errors"
	"fmt"
	"io"
	"os"
)

func main() {
	file, err := os.OpenFile("Resource/test.txt", os.O_RDONLY, 0666)
	if os.IsNotExist(err) {
		fmt.Println("File does not exist")
	} else if err != nil {
		fmt.Println("Error reading file")
	} else {
		defer file.Close()
		fmt.Println("File reading success")
		bytes := make([]byte, 1024)
		bytes, err = ReadFile(file)
		if err != nil {
			fmt.Println("Error reading file")
		}
		fmt.Println(string(bytes))
	}

}

func ReadFile(file *os.File) ([]byte, error) {
	buffer := make([]byte, 0, 512)
	for {
		if len(buffer) == cap(buffer) {
			buffer = append(buffer, 0)[:len(buffer)]
		}
		// Read
		offset, err := file.Read(buffer[len(buffer):cap(buffer)])
		buffer = buffer[:len(buffer)+offset]
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return buffer, err
		}
	}
}
