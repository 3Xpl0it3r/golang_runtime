package runtime

import (
	"context"
	"sync"
	"time"
)

func go源码阅读_调度器(){
	checkTimers()
	libpreinit()

	/*
							调度器的初始化
	 */

	// 调度器
	// 调度器初始化p, 尝试p和m绑定
	schedinit()			// 初始化调度器， 根据cpu的个数来创建处理器(缩容/扩容)， 主要通过 procresize 函数来实现的
	// processsize 创建/初始化合适的数量的处理器，非空p(p队列里面存在非空的g), 尝试获取一个空闲的m 将m和p进行绑定起来

	newproc() 			// d当检测到有go关键字的时候，编译器会将go语句转换成newproc函数调用， 这个时候会获取/创建一个goroutine 结构体，并且初始化，设置堆栈信息，设置一些调度信息，然后将goroutine 放到队列里面，等待调度


	/*
						调度器的执行
	 */

	mstart() 			// 调度器启动之后由runtime来运行，调用mstart() 初始化g0栈信息， mstart1 初始化线程，并且调用runtime.scheduler进入调度循环
	schedule()			// mstart 最终会调用schedule 函数，最终一直在调度循环里面
	/*
	调度逻辑
	1。 一定的概率从全局队列里面检查/检索对应的goroutine
	2。 从处理器本地队列里面检索
	3。 调用findrunnable 检索(本地队列， netepool, runqsteal偷) ---> 这个步骤一定会get到一个goroutine的

	开始调用execute 执行goroutine , execute是一个死循环
	 */
	var gp *g
	execute(gp, true) 		// 将gp调度到当前的m上执行， 这个函数永远不会退出
	/*
		execute 函数 执行用户的函数，在调度之前就将sched.SP 设置为了goexit, sched.PC 设置为fn, 根据go的调用约定，当函数返回的时候会跳转到SP地址上执行
		也就是当用户程序退出的时候，这个时候会继续执行goexit()函数。 goexit是一个汇编指令，最终调用goexit1
	 */
	goexit1() // goexist1 本质会跳转到g0的栈上去调用goexit0, goexit0函数的唯一作用就清理goroutine,然后将goroutine重新放到p的freelist里面，等待服用，最终重新进度调度器
	/*
		最终调用逻辑图
		schedule ----> get routine -----> execute ------> user fn --------> goexit0 -------|
	       /|\																		       |
	       |																               |
	       |---------------------------------------------------------------------------------
	 */


	/*
						调度器的触发
	 */
	/*
	触发的逻辑/触发的时间点

	系统监控			semrelease  --------->  semrelease1 ------> goyield ---------> goyield_m
	协作式调度 		godched		--------->  gosched_m -----> 					goschedImpl
	系统调用 		exitsyscall ---------> 	exitsyscall0
	主动挂起			gopark 		---------> 					park_m

	上面提到的mstart 触发
	 */
	源码阅读_调度器_调度时间点_主动挂起() 			// 主动挂起是最常见的触发方式，这种方式会将当前goroutine暂停，被暂停的任务不会放到运行队列里面
	源码阅读_调度器_调度时间点_系统调用() 			//


	/*
					线程管理
	 */
	// go语言runtime通过调度器改变线程所有权，提供了runtime.LockOSThread 和runtime.UnlockOSThread 来完成一些goroutine和线程的绑定操作
	//  将goroutine和线程绑定
	LockOSThread()  // 绑定
	UnlockOSThread() // 解除绑定
	// 一般cgo会用得多，正常情况用不到



	/*
					线程的生命周期
	 */
	// go语言通过runtime.startm启动线程来执行处理器，如果没有从freelist里面获取到线程，就会调用runtime.newm 来创建新线程
	newm()

}


func 源码阅读_调度器_调度时间点_主动挂起(){
	// Puts the current goroutine into a waiting state and calls unlockf , 它会将当前的goroutine放到等待队列里面去，然后调用unlockf
	gopark() 	//  gopark代码 将当前goroutine和m解除绑定，更新m一些信息，最终在g0栈上调用park_m 函数

	var gp *g
	park_m(gp) 		///  park_m 在g0栈上执行，解除当前goroutine 和 m的绑定关系,将当前goroutine设置为waiting, 然后执行unlockf ，执行完，重新进入调度

	// 恢复工作
	//  当goroutine等待条件满足后，runtime 会调用runtime.goready 函数将因为调用gopark而陷入休眠的goroutine 唤醒
	goready(nil, timerDeleted)
	// 这个函数最终会在系统栈上执行ready()函数
	ready()
}


func 源码阅读_调度器_调度时间点_系统调用(){
	// syscall 会触发系统重新调度， go通过Syscall/RawSyscall 封装了系统调用
	// 通过汇编对 syscall做了封装，Syscall 最终会调用runtime.syscall函数， RawSyscall 会最简化，不会触发， 汇编代码位置: src/syscall/asm_<arch>.s
	// 可以立刻返回的系统调用被设置成RawSyscall,  会阻塞的系统调用会被设置成Syscall
	/*
				SysCall 					RawSyscall
				  |								|
				enetersyscall 					|
					|							|
				  SYSCALL 						|
					|							|
				exitsyscall 					|
	 */
	// 进入系统调用
	entersyscall()		//  entersyscall 会调用reentersyscall 函数来真正的进入系统调用， reentersyscall 本质做的工作就是暂停goroutine  分离goroutine和m 当抢占的处理器状态设置为PSyscall ，
	// 设置完这些，通过汇编指令，再去执行syscall
	// MOVQ	trap+0(FP), AX	// syscall entry

	// 系统调用完成后执行恢复工作 CALL	runtime·exitsyscall(SB)
	exitsyscall()
	// exitsyscall 查找可用的处理器P来执行当前的goroutine


	// case 1
	// 优先通过exitsyscallfast 函数来快速查找
	exitsyscallfast() // 通过这个函数来快速检测之前的处理器是否可用,如果没有尝试获取一个空闲的处理器
	{
		// 在exitsyscallfastl里面有两个分钟
		// case 1 如果goroutine原先处理器处于syscall状态，直接调用wirep 将goroutine 与处理器关联
		{
			wirep(oldp)
			exitsyscallfast_reacquired()  // 将goroutine 和原先处理器关联
			 // return true
		}
		// case2 否则，尝试exitsyscallfast_pidle函数---> acquire尝试使用闲置处理器来处理当前的goroutine
		exitsyscallfast_pidle()
		{
			pidleget()
			acquirep()
		}

		// 获取到p
		Gosched() 	// 获取到p后开始进行调度, 可用则更新goroutine调度信息，然后执行Gosched函数
		{
			gosched_m() // Gosched 最终会在g0栈上执行gosched_m()函数，
			goschedImpl()   // gosched_m()最终会调用goschedImpl() 函数将goroutine重新放到队列上面，触发新一轮的调度
		}
	}

	// case2 如果没有找到，则执行下么函数
	mcall(exitsyscall0)
	{
		// 更新goroutine的状态
		// 寻找空闲处理器， 寻找到就执行execute函数(), 执行完重新触发下一轮调度

		// 如果没有寻找到空闲的处理器，那么就将goroutine 重新放到全局队列里面，然后重新开始一轮新的调度
	}
}

func 源码阅读_调度器_调度时间点_协作式调度(){
	// 在系统调用里面触发调度，里面已经包含了协作式的调度
	// 协作式的调度会主动让出处理器，允许其他goroutine运行。该函数无法挂起goroutine, 调度器可能会将当前goroutine调度到其他的节点
	Gosched()
	{
		mcall(gosched_m)
		{
			goschedImpl()
			{
					// 具体查看函数注释
			}
		}
	}
}