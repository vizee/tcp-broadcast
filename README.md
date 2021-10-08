# tcp-broadcast

虽然是写出来了，但手头只有一个 5.4 的 kernel，代码没测过，就这样吧🤷‍♂️

## 小作文

`internal/sys` 是对需要用到的 syscall 和相关类型、常量封装，用 cgo 生成的 go 定义；`internal/yaur` 是对 io-uring 的封装，属于是模板代码了（低配 liburing？🤷‍♂️

主要的代码也就 [main.go:52](/main.go#L52) 那么一段，剩下的都是各种封装，花点功夫去完善一下的话，是有可能做成一个比较完整的 io 库，实际上一开始的目标是这样的，然后我选择了放过自己

[closeConn 函数](/main.go#L17) 稍微展开说说。在实现 closeConn 功能的时候，需要先用 IORING_OP_ASYNC_CANCEL 取消异步队列里的 IO 请求，再释放内存（从根对象引用移除），在我一开始的思路里实现这个功能需要引入复杂的机制，因为取消操作可能失败的原因，我需要额外维护 pd 来跟随取消操作的结果，但事实上，因为 conn 的 rpd 和 wpd 是结构体类型，只要引用它们就不会回收 conn 的内存，然后我给 rpd 和 wpd 创建对应的取消操作的 pd，并且把所有 pd 维护起来，就能保证内存能在正确的时间释放，然后通过 finalizer 机制，推迟 close fd 的时间点
