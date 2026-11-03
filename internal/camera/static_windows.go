package camera

// The DirectShow capture code is C++: link MinGW's C++ and GCC runtimes
// into the executable, so it doesn't need libstdc++-6.dll and
// libgcc_s_seh-1.dll next to it.

// #cgo windows LDFLAGS: -static -static-libgcc -static-libstdc++
import "C"
