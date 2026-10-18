package main

import (
 "os"
 "syscall"
 "unsafe"
)

func main() {
 pointer := unsafe.Pointer(syscall.GetCommandLine())
 var data []uint16
 for index := 0; ; index++ {
  value := *(*uint16)(unsafe.Add(pointer, index*2))
  if value == 0 { break }
  data = append(data, value)
 }
 if err := os.WriteFile(os.Getenv("PRIME_COMMAND_CAPTURE"), []byte(syscall.UTF16ToString(data)), 0600); err != nil { os.Exit(1) }
}
