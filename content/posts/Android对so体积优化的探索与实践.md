---
title: "Android对so体积优化的探索与实践"
date: 2022-07-19T04:09:13+0000
tags: [美团技术团队, 美团平台, 前端, Android, so, Optimization]
---

## 1. 背景


应用安装包的体积影响着用户的下载时长、安装时长、磁盘占用空间等诸多方面，因此减小安装包的体积对于提升用户体验和下载转化率都大有益处。Android 应用安装包其实是一个 zip 文件，主要由 dex、assets、resource、so 等各类型文件压缩而成。目前业内常见的包体积优化方案大体分为以下几类：



* 针对 dex 的优化，例如 Proguard、dex 的 DebugItem 删除、字节码优化等；
* 针对 resource 的优化，例如 AndResGuard、webp 优化等；
* 针对 assets 的优化，例如压缩、动态下发等；
* 针对 so 的优化，同 assets，另外还有移除调试符号等。


随着动态化、端智能等技术的广泛应用，在采用上述优化手段后， so 在安装包体积中的比重依然很高，我们开始思索这部分体积是否能进一步优化。



经过一段时间的调研、分析和验证，我们逐渐摸索出一套可以将应用安装包中 so 体积进一步减小 30%～60% 的方案。该方案包含一系列纯技术优化手段，对业务侵入性低，通过简单的配置，可以快速部署生效，目前美团 App 已在线上部署使用。为让大家能知其然，也能知其所以然，本文将先从 so 文件格式讲起，结合文件格式分析哪些内容可以优化。



## 2. so 文件格式分析


so 即动态库，本质上是 ELF（Executable and Linkable Format）文件。可以从两个维度查看 so 文件的内部结构：链接视图（Linking View）和执行视图（Execution View）。链接视图将 so 主体看作多个 section 的组合，该视图体现的是 so 是如何组装的，是编译链接的视角。而执行视图将 so 主体看作多个 segment 的组合，该视图告诉动态链接器如何加载和执行该 so，是运行时的视角。鉴于对 so 优化更侧重于编译链接角度，并且通常一个 segment 包含多个 section（即链接视图对 so 的分解粒度更小），因此我们这里只讨论 so 的链接视图。



通过 `readelf -S` 命令可以查看一个 so 文件的所有 section 列表，参考 ELF 文件格式说明，这里简要介绍一下本文涉及的 section：



* `.text`：存放的是编译后的机器指令，C/C\+\+代码的大部分函数编译后就存放在这里。这里只有机器指令，没有字符串等信息。
* `.data`：存放的是初始值不为零的一些可读写变量。
* `.bss`：存放的是初始值为零或未初始化的一些可读写变量。该 section 仅指示运行时需要的内存大小，不会占用 so 文件的体积。
* `.rodata`：存放的是一些只读常量。
* `.dynsym`：动态符号表，给出了该 so 对外提供的符号（导出符号）和依赖外部的符号（导入符号）的信息。
* `.dynstr`：字符串池，不同字符串以 ‘\\0’ 分割，供 `.dynsym` 和其他部分使用。
* `.gnu.hash` 和`.hash`：两种类型的哈希表，用于快速查找 `.dynsym` 中的导出符号或全部符号。
* `.gnu.version`、`.gnu.version_d`、`.gnu.version_r`：这三个 section 用于指定动态符号表中每个符号的版本，其中`.gnu.version` 是一个数组，其元素个数与动态符号表中符号的个数相同，即数组每个元素与动态符号表的每个符号是一一对应的关系。数组每个元素的类型为 `Elfxx_Half`，其意义是索引，指示每个符号的版本。`.gnu.version_d` 描述了该 so 定义的所有符号的版本，供`.gnu.version` 索引。`.gnu.version_r` 描述了该 so 依赖的所有符号的版本，也供 `.gnu.version` 索引。因为不同的符号可能具有相同的版本，所以采用这种索引结构，可以减小 so 文件的大小。


在进行优化之前，我们需要对这些 section 以及它们之间的关系有一个清晰的认识，下图较直观地展示了 so 中各个 section 之间的关系（这里只绘制了本文涉及的 section）：



![图1 so文件结构示意图](https://p1.meituan.net/travelcube/ef5625f83d3efbde4d4d6433c3be3e9c317797.png)



结合上图，我们从另一个角度来理解 so 文件的结构：想象一下，我们把所有的函数实现体都放到`.text` 中，`.text` 中的指令会去读取 `.rodata` 中的数据，读取或修改 `.data` 和 `.bss` 中的数据。看上去 so 中有这些内容也足够了。但是这些函数怎样执行呢？也就是说，只把这些函数和数据加载进内存是不够的，这些函数只有真正去执行，才能发挥作用。



我们知道想要执行一个函数，只要跳转到它的地址就行了。那外界调用者（该 so 之外的模块）怎样知道它想要调用函数的地址呢？这里就涉及一个函数 ID 的问题：外部调用者给出需要调用的函数的 ID，而动态链接器（Linker）根据该 ID 查找目标函数的地址并告知外部调用者。所以 so 文件还需要一个结构去存储“ID\-地址”的映射关系，这个结构就是动态符号表的所有导出符号。



具体到动态符号表的实现，ID 的类型是“字符串”，可以说动态符号表的所有导出符号构成了一个“字符串\-地址“的映射表。调用者获取目标函数的地址后，准备好参数跳转到该地址就可以执行这个函数了。另一方面，当前 so 可能也需要调用其他 so 中的函数（例如 libc.so 中的 read、write 等），动态符号表的导入符号记录了这些函数的信息，在 so 内函数执行之前动态链接器会将目标函数的地址填入到相应位置，供该 so 使用。所以动态符号表是连接当前 so 与外部环境的“桥梁”：导出符号供外部使用，导入符号声明了该 so 需要使用的外部符号（注：实际上 `.dynsym` 中的符号还可以代表变量等其他类型，与函数类型类似，这里就不再赘述）。



结合 so 文件结构，接下来我们开始分析 so 中有哪些内容可以优化。



## 3. so 可优化内容分析


在讨论 so 可优化内容之前，我们先了解一下 Android 构建工具（Android Gradle Plugin，下文简称 AGP）对 so 体积做的 strip 优化（移除调试信息和符号表）。AGP 编译 so 时，首先产生的是带调试信息和符号表的 so（任务名为 externalNativeBuildRelease），之后对刚产生的带调试信息和符号表的 so 进行 strip，就得到了最终打包到 apk 或 aar 中的 so（任务名为 stripReleaseDebugSymbols）。



strip 优化的作用就是删除输入 so 中的调试信息和符号表。这里说的符号表与上文中的“动态符号表”不同，符号表所在 section 名通常为 .symtab，它通常包含了动态符号表中的全部符号，并且额外还有很多符号。调试信息顾名思义就是用于调试该 so 的信息，主要是各种名字以 `.debug_` 开头的 section，通过这些 section 可以建立 so 每条指令与源码文件的映射关系（也就是能够对 so 中每条指令找到其对应的源码文件名、文件行号等信息）。 之所以叫 strip 优化，是因为其实际调用的是 NDK 提供的的 strip 命令（所用参数为–strip\-unneeded）。



> 注：为什么 AGP 要先编译出带调试信息和符号表的 so，而不直接编译出最终的 so 呢（通过添加 `-s` 参数是可以做到直接编译出没有调试信息和符号表的 so 的）？原因就在于需要使用带调试信息和符号表的 so 对崩溃调用栈进行还原。删除了调试信息和符号表的 so 完全可以正常运行，但是当它发生崩溃时，只能保证获取到崩溃调用栈的每个栈帧的相应指令在 so 中的位置，不一定能获取到符号。但是排查崩溃问题时，我们希望得知 so 崩溃在源码的哪个位置。带调试信息和符号表的 so 可以将崩溃调用栈的每个栈帧还原成其对应的源码文件名、文件行号、函数名等，大大方便了崩溃问题的排查。所以说，虽然带调试信息和符号表的 so 不会打包到最终的 apk 中，但它对排查问题来说非常重要。


AGP 通过开启 strip 优化，可以大幅缩减 so 的体积，甚至可以达到十倍以上。以一个测试 so 为例，其最终 so 大小为14 KB，但是对应的带调试信息和符号表的 so 大小为 136 KB。不过在使用中，我们需要注意的是，如果 AGP 找不到对应的 strip 命令，就会把带调试信息和符号表的 so 直接打包到 apk 或 aar 中，并不会打包失败。例如缺少 armeabi 架构对应的 strip 命令时提示信息如下：



```
Unable to strip library 'XXX.so' due to missing strip tool for ABI 'ARMEABI'. Packaging it as is.
```


除了上述 Android 构建工具默认为 so 体积做的优化，我们还能做哪些优化呢？首先明确我们优化的原则：



* 对于必须保留的内容考虑进行缩减，减小体积占用；
* 对于无需保留的内容直接删除。


基于以上原则，可以从以下三个方面对 so 继续进行深入优化：



* **精简动态符号表**：上文已经提到，动态符号表是 so 与外部进行连接的“桥梁”，其中的导出表相当于是 so 对外暴露的接口。哪些接口是必须对外暴露的呢？在 Android 中，大部分 so 是用来实现 Java 的 native 方法的，对于这种 so，只要让应用运行时能够获取到 Java native 方法对应的函数地址即可。要实现这个目标，有两种方法：一种是使用 RegisterNatives 动态注册 Java native 方法，一种是按照 JNI 规范定义 `java_***` 样式的函数并导出其符号。RegisterNatives 方式可以提前检测到方法签名不匹配的问题，并且可以减少导出符号的数量，这也是 Google 推荐的做法。所以在最优情况下只需导出 `JNI_OnLoad`（在其中使用 RegisterNatives 对 Java native 方法进行动态注册）和 `JNI_OnUnload`（可以做一些清理工作）这两个符号即可。如果不希望改写项目代码，也可以再导出 `java_***` 样式的符号。除了上述类型的 so，剩余的 so 通常是被应用的其他 so 动态依赖的，对于这类 so，需要确定所有动态依赖它的 so 依赖了它的哪些符号，仅保留这些被依赖的符号即可。另外，这里应区分符号表项与实现体，符号表项是动态符号表中相应的 `Elfxx_Sym` 项（见上图），实现体是其在 `.text`、`.data`、 `.bss`、`.rodata` 等或其他部分的实体。删除了符号表项，实现体不一定要被删除。结合上文 so 文件结构示意图，可以预估出删除一个符号表项后 so 减小的体积为：符号名字符串长度\+ 1 \+ `Elfxx_Sym` \+ `Elfxx_Half` \+ `Elfxx_Word` 。
* **移除无用代码**：在实际的项目中，有一些代码在 Release 版中永远不会被使用到（例如历史遗留代码、用于测试的代码等），这些代码被称为 DeadCode。而根据上文分析，只有动态符号表的导出符号直接或间接引用到的所有代码才需要保留，其他剩余的所有代码都是 DeadCode，都是可以删除的（注：事实上 `.init_array` 等特殊 section 涉及的代码也要保留）。删除无用代码的潜在收益较大。
* **优化指令长度**：实现某个功能的指令并不是固定的，编译器有可能能用更少的指令完成相同的功能，从而实现优化。由于指令是 so 的主要组成部分，因此优化这一部分的潜在收益也比较大。


so 可优化内容如下图所示（可删除部分用红色背景标出，可优化部分是 `.text`），其中 funC、value2、value3、value6 由于分别被需保留部分使用，所以需要保留其实现体，只能删除其符号表项。funD、value1、value4、value5 可删除符号表项及其实现体（注：因为 value4 的实现体在 `.bss` 中，而 `.bss` 实际不占用 so 的体积，所以删除 value4 的实现体不会减小 so 的体积）。



![图2 so可优化部分](https://p0.meituan.net/travelcube/8d6f6d906141265110dbbc6325b66a64317410.png)



在确定了 so 中可以优化的内容后，我们还需要考虑优化时机的问题：是直接修改 so 文件，还是控制其生成过程？考虑到直接修改 so 文件的风险与难度较大，控制 so 的生成过程显然更稳妥。为了控制 so 的生成过程，我们先简要介绍一下 so 的生成过程：



![图3 so文件的生成过程](https://p0.meituan.net/travelcube/4a0a8066d47b408e830d3039c917766a17947.png)



如上图所示，so 的生成过程可以分为四个阶段：



* **预处理**：将 include 头文件处扩展为实际文件内容并进行宏定义替换。
* **编译**：将预处理后的文件编译成汇编代码。
* **汇编**：将汇编代码汇编成目标文件，目标文件中包含机器指令（大部分情况下是机器指令，见下文 LTO 一节）和数据以及其他必要信息。
* **链接**：将输入的所有目标文件以及静态库（.a 文件）链接成 so 文件。


可以看出，预处理和汇编阶段对特定输入产生的输出基本是固定的，优化空间较小。所以我们的优化方案主要是针对编译和链接阶段进行优化。



## 4. 优化方案介绍


我们对所有能控制最终 so 体积的方案都进行调研，并验证了其效果，最后总结出较为通用的可行方案。



### 4.1 精简动态符号表


#### 使用 visibility 和 attribute 控制符号可见性


可以通过给编译器传递 `-fvisibility=VALUE` 控制全局的符号可见性，VALUE 常取值为 default 和 hidden：



* **default**：除非对变量或函数特别指定符号可见性，所有符号都在动态符号表中，这也是不使用 \-fvisibility 时的默认值。
* **hidden**：除非对变量或函数特别指定符号可见性，所有符号在动态符号表中都不可见。


CMake 项目的配置方式：



```
set(CMAKE_C_FLAGS "${CMAKE_C_FLAGS} -fvisibility=hidden")
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -fvisibility=hidden")
```


ndk\-build 项目的配置方式：



```
LOCAL_CFLAGS += -fvisibility=hidden
```


另一方面，针对单个变量或函数，可以通过 attribute 方式指定其符号可见性，示例如下：



```C
__attribute__((visibility("hidden")))
int hiddenInt=3;
```


其常用值也是 default 和 hidden，与 visibility 方式意义类似，这里不再赘述。



attribute 方式指定的符号可见性的优先级，高于 visibility 方式指定的可见性，相当于 visibility 是全局符号可见性开关，attribute 方式是针对单个符号的可见性开关。这两种方式结合就能控制源码中每个符号的可见性。



需要注意的是上面这两种方式，只能控制变量或函数是否存在于动态符号表中（即是否删除其动态符号表项），而不会删除其实现体。



#### 使用 static 关键字控制符号可见性


在C/C\+\+语言中，static 关键字在不同场景下有不同意义，当使用 static 表示“该函数或变量仅在本文件可见”时，那么这个函数或变量就不会出现在动态符号表中，但只会删除其动态符号表项，而不会删除其实现体。static 关键字相当于是增强的 hidden（因为 static 声明的函数或变量编译时只对当前文件可见，而 hidden 声明的函数或变量只是在动态符号表中不存在，在编译期间对其他文件还是可见的）。在项目开发中，使用 static 关键字声明一个函数或变量“仅在本文件可见”是很好的习惯，但是不建议使用 static 关键字控制符号可见性：无法使用 static 关键字控制一个多文件可见的函数或变量的符号可见性。



#### 使用 exclude libs 移除静态库中的符号


上述 visibility 方式、attribute 方式和 static 关键字，都是控制项目源码中符号的可见性，而无法控制依赖的静态库中的符号在最终 so 中是否存在。exclude libs 就是用来控制依赖的静态库中的符号是否可见，它是传递给链接器的参数，可以使依赖的静态库的符号在动态符号表中不存在。同样，也是只能删除符号表项，实现体仍然会存在于产生的 so 文件中。



CMake 项目的配置方式：



```
set(CMAKE_SHARED_LINKER_FLAGS "${CMAKE_SHARED_LINKER_FLAGS} -Wl,--exclude-libs,ALL")#使所有静态库中的符号都不被导出
set(CMAKE_SHARED_LINKER_FLAGS "${CMAKE_SHARED_LINKER_FLAGS} -Wl,--exclude-libs,libabc.a")#使 libabc.a 的符号都不被导出
```


ndk\-build 项目的配置方式：



```
LOCAL_LDFLAGS += -Wl,--exclude-libs,ALL #使所有静态库中的符号都不被导出
LOCAL_LDFLAGS += -Wl,--exclude-libs,libabc.a #使 libabc.a 的符号都不被导出
```


#### 使用 version script 控制符号可见性


version script 是传递给链接器的参数，用来指定动态库导出哪些符号以及符号的版本。该参数会影响到上面“so 文件格式”一节中 `.gnu.version` 和 `.gnu.version_d` 的内容。我们现在只使用它的指定所有导出符号的功能（即符号版本名使用空字符串）。开启 version script 需要先编写一个文本文件，用来指定动态库导出哪些符号。示例如下（只导出 usedFun 这一个函数）：



```
{
    global:usedFun;
    local:*;
};
```


然后将上述文件的路径传递给链接器即可（假定上述文件名为 `version_script.txt`）。



CMake 项目的配置方式：



```
set(CMAKE_SHARED_LINKER_FLAGS "${CMAKE_SHARED_LINKER_FLAGS} -Wl,--version-script=${CMAKE_CURRENT_SOURCE_DIR}/version_script.txt") #version_script.txt 与当前 CMakeLists.txt 同目录
```


ndk\-build 项目的配置方式：



```
LOCAL_LDFLAGS += -Wl,--version-script=${LOCAL_PATH}/version_script.txt #version_script.txt 与当前 Android.mk 同目录
```


看上去，version script 是明确地指定需要保留的符号，如果通过 visibility 结合 attribute 的方式控制每个符号是否导出，也能达到 version script 的效果，但是 version script 方式有一些额外的好处：



1. version script 方式可以控制编译进 so 的静态库的符号是否导出，visibility 和 attribute 方式都无法做到这一点。
2. visibility 结合 attribute 方式需要在源码中标明每个需要导出的符号，对于导出符号较多的项目来说是很繁杂的。version script 把需要导出的符号统一地放到了一起，能够直观方便地查看和修改，对导出符号较多的项目也非常友好。
3. version script 支持通配符，`*` 代表0个或者多个字符，`?` 代表单个字符。比如 `my*;` 就代表所有以 my 开头的符号。有了通配符的支持，配置 version script 会更加方便。
4. 还有非常特殊的一点，version script 方式可以删除 `__bss_start` 这样的一些符号（这是链接器默认加上的符号）。


综上所述，version script 方式优于 visibility 结合 attribute 的方式。同时，使用了 version script 方式，就不需要使用 exclude libs 方式控制依赖的静态库中的符号是否导出了。



### 4.2 移除无用代码


#### 开启 LTO


LTO 是 Link Time Optimization 的缩写，即链接期优化。LTO 能够在链接目标文件时检测出 DeadCode 并删除它们，从而减小编译产物的体积。DeadCode 举例：某个 if 条件永远为假，那么 if 为真下的代码块就可以移除。进一步地，被移除代码块所调用的函数也可能因此而变为 DeadCode，它们又可以被移除。能够在链接期做优化的原因是，在编译期很多信息还不能确定，只有局部信息，无法执行一些优化。但是链接时大部分信息都确定了，相当于获取了全局信息，所以可以进行一些优化。GCC 和 Clang 均支持 LTO。LTO 方式编译的目标文件中存储的不再是具体机器的指令，而是机器无关的中间表示（GCC 采用的是 GIMPLE 字节码，Clang 采用的是 LLVM IR 比特码）。



CMake 项目的配置方式：



```
set(CMAKE_C_FLAGS "${CMAKE_C_FLAGS} -flto")
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -flto")
set(CMAKE_SHARED_LINKER_FLAGS "${CMAKE_SHARED_LINKER_FLAGS} -O3 -flto")
```


ndk\-build 项目的配置方式：



```
LOCAL_CFLAGS += -flto
LOCAL_LDFLAGS += -O3 -flto
```


使用 LTO 时需要注意几点：



1. 如果使用 Clang，编译参数和链接参数中都要开启 LTO，否则会出现无法识别文件格式的问题（NDK22 之前存在此问题）。使用 GCC 的话，只需要编译参数中开启 LTO 即可。
2. 如果项目工程依赖了静态库，可以使用 LTO 方式重新编译该静态库，那么编译动态库时，就能移除静态库中的 DeadCode，从而减小最终 so 的体积。
3. 经过测试，如果使用 Clang，链接器需要开启非 0 级别的优化，LTO 才能真正生效。经过实际测试（NDK 为 r16b），O1 优化效果较差，O2、O3 优化效果比较接近。
4. 由于需要进行更多的分析计算，开启 LTO 后，链接耗时会明显增加。


#### 开启 GC sections


这是传递给链接器的参数，GC 即 Garbage Collection（垃圾回收），也就是对无用的 section 进行回收。注意，这里的 section 不是指最终 so 中的 section，而是作为链接器的输入的目标文件中的 section。



简要介绍一下目标文件，目标文件（扩展名 .o ）也是 ELF 文件，所以也是由 section 组成的，只不过它只包含了相应源文件的内容：函数会放到 `.text` 样式的 section 中，一些可读写变量会放到 `.data` 样式的 section 中，等等。链接器会把所有输入的目标文件的同类型的 section 进行合并，组装出最终的 so 文件。



GC sections 参数通知链接器：仅保留动态符号（及 `.init_array` 等）直接或者间接引用到的 section，移除其他无用 section。这样就能减小最终 so 的体积。但开启 GC sections 还需要考虑一个问题：编译器默认会把所有函数放到同一个 section 中，把所有相同特点的数据放到同一个 section 中，如果同一个 section 中既有需要删除的部分又有需要保留的部分，会使得整个 section 都要保留。所以我们需要减小目标文件 section 的粒度，这需要借助另外两个编译参数 `-fdata-sections` 和 `-ffunction-sections` ，这两个参数通知编译器，将每个变量和函数分别放到各自独立的 section 中，这样就不会出现上述问题了。实际上 Android 编译目标文件时会自动带上 `-fdata-sections` 和 `-ffunction-sections` 参数，这里一并列出来，是为了突出它们的作用。



CMake 项目的配置方式：



```
set(CMAKE_C_FLAGS "${CMAKE_C_FLAGS} -fdata-sections -ffunction-sections")
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -fdata-sections -ffunction-sections")
set(CMAKE_SHARED_LINKER_FLAGS "${CMAKE_SHARED_LINKER_FLAGS} -Wl,--gc-sections")
```


ndk\-build 项目的配置方式：



```
LOCAL_CFLAGS += -fdata-sections -ffunction-sections
LOCAL_LDFLAGS += -Wl,--gc-sections
```


### 4.3 优化指令长度


#### 使用 Oz/Os 优化级别


编译器根据输入的 \-Ox 参数决定编译的优化级别，其中 O0 表示不开启优化（这种情况主要是为了便于调试以及更快的编译速度），从 O1 到 O3，优化程度越来越强。Clang 和 GCC 均提供了 Os 的优化级别，其与 O2 比较接近，但是优化了生成产物的体积。而 Clang 还提供了 Oz 优化级别，在 Os 的基础上能进一步优化产物体积。



综上，编译器是 Clang，可以开启 Oz 优化。如果编译器是 GCC，则只能开启 Os 优化（注：NDK 从 r13 开始默认编译器从 GCC 变为 Clang，r18 中正式移除了 GCC。GCC 不支持 Oz 是指 Android 最后使用的 GCC4.9 版本不支持 Oz 参数）。Oz/Os 优化相比于 O3 优化，优化了产物体积，性能上可能有一定损失，因此如果项目原本使用了 O3 优化，可根据实际测试结果以及对性能的要求，决定是否使用 Os/Oz 优化级别，如果项目原本未使用 O3 优化级别，可直接使用 Os/Oz 优化。



CMake 项目的配置方式（如果使用 GCC，应将 Oz 改为 Os）：



```
set(CMAKE_C_FLAGS "${CMAKE_C_FLAGS} -Oz")
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -Oz")
```


ndk\-build 项目的配置方式（如果使用 GCC，应将 Oz 改为 Os）：



```
LOCAL_CFLAGS += -Oz
```


### 4.4 其他措施


#### 禁用 C\+\+ 的异常机制


如果项目中没有使用 C\+\+ 的异常机制（例如 `try...catch` 等），可以通过禁用 C\+\+ 的异常机制，来减小 so 的体积。



CMake 项目的配置方式：



```
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -fno-exceptions")
```


ndk\-build 默认会禁用 C\+\+ 的异常机制，因此无需特意禁用（如果现有项目开启了 C\+\+ 的异常机制，说明确有需要，需仔细确认后才能禁用）。



#### 禁用 C\+\+ 的 RTTI 机制


如果项目中没有使用 C\+\+ 的 RTTI 机制（例如 typeid 和 dynamic\_cast 等），可以通过禁用 C\+\+ 的 RTTI ，来减小 so 的体积。



CMake 项目的配置方式：



```
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -fno-rtti")
```


ndk\-build 默认会禁用 C\+\+ 的 RTTI 机制，因此无需特意禁用（如果现有项目开启了 C\+\+ 的 RTTI 机制，说明确有需要，需仔细确认后才能禁用）。



#### 合并 so


以上都是针对单个 so 的优化方案，对单个 so 进行优化后，还可以考虑对 so 进行合并，能够进一步减小 so 的体积。具体来讲，当安装包内某些 so 仅被另外一个 so 动态依赖时，可以将这些 so 合并为一个 so。例如 liba.so 和 libb.so 仅被 libx.so 动态依赖，可以将这三个 so 合并为一个新的 libx.so。合并 so 有以下好处：



1. 可以删除部分动态符号表项，减小 so 总体积。具体来讲，就是可以删除 liba.so 和 libb.so 的动态符号表中的所有导出符号，以及 libx.so 的动态符号表中从 liba.so 和 libb.so 中导入的符号。
2. 可以删除部分 PLT 表项和 GOT 表项，减小 so 总体积。具体来讲，就是可以删除 libx.so 中与 liba.so、libb.so 相关的 PLT 表项和 GOT 表项。
3. 可以减轻优化的工作量。如果没有合并 so，对 liba.so 和 libb.so 做体积优化时需要确定 libx.so 依赖了它们的哪些符号，才能对它们进行优化，做了 so 合并后就不需要了。链接器会自动分析引用关系，保留使用到的所有符号的对应内容。
4. 由于链接器对原 liba.so 和 libb.so 的导出符号拥有了更全的上下文信息，LTO 优化也能取得更好的效果。


可以在不修改项目源码的情况下，在编译层面实现 so 的合并。



#### 提取多 so 共同依赖库


上面“合并 so”是减小 so 总个数，而这里是增加 so 总个数。当多个 so 以静态方式依赖了某个相同的库时，可以考虑将此库提取成一个单独的 so，原来的几个 so 改为动态依赖该 so。例如 liba.so 和 libb.so 都静态依赖了 libx.a，可以优化为 liba.so 和 libb.so 均动态依赖 libx.so。提取多 so 共同依赖库，可以对不同 so 内的相同代码进行合并，从而减小总的 so 体积。



这里典型的例子是 libc\+\+ 库：如果存在多个 so 都静态依赖 libc\+\+ 库的情况，可以优化为这些 so 都动态依赖于 `libc++_shared.so`。



### 4.5 整合后的通用方案


通过上述分析，我们可以整合出普通项目均可使用的通用的优化方案，CMake 项目的配置方式（如果使用 GCC，应将 Oz 改为 Os）：



```
set(CMAKE_C_FLAGS "${CMAKE_C_FLAGS} -Oz -flto -fdata-sections -ffunction-sections")
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -Oz -flto -fdata-sections -ffunction-sections")
set(CMAKE_SHARED_LINKER_FLAGS "${CMAKE_SHARED_LINKER_FLAGS} -O3 -flto  -Wl,--gc-sections -Wl,--version-script=${CMAKE_CURRENT_SOURCE_DIR}/version_script.txt") #version_script.txt 与当前 CMakeLists.txt 同目录
```


ndk\-build 项目的配置方式（如果使用 GCC，应将 Oz 改为 Os）：



```
LOCAL_CFLAGS += -Oz -flto -fdata-sections -ffunction-sections
LOCAL_LDFLAGS += -O3 -flto -Wl,--gc-sections -Wl,--version-script=${LOCAL_PATH}/version_script.txt #version_script.txt 与当前 Android.mk 同目录
```


其中 `version_script.txt` 较为通用的配置如下，可根据实际情况添加需要保留的导出符号：



```
{
    global:JNI_OnLoad;JNI_OnUnload;Java_*;
    local:*;
};
```


**说明**：version script 方式指定所有需要导出的符号，不再需要 visibility 方式、attribute 方式、static 关键字和 exclude libs 方式控制导出符号。是否禁用 C\+\+ 的异常机制和 RTTI 机制、合并 so 以及提取多 so 共同依赖库取决于具体项目，不具有通用性。



至此，我们总结出一套可行的 so 体积优化方案。但在工程实践中，还有一些问题要解决。



## 5. 工程实践


### 支持多种构建工具


美团有众多业务使用了 so，所使用的构建工具也不尽相同，除了上述常见的 CMake 和 ndk\-build，也有项目在使用 Make、Automake、Ninja、GYP 和 GN 等各种构建工具。不同构建工具应用 so 优化方案的方式也不相同，尤其对大型工程而言，配置复杂性较高。



基于以上原因，每个业务自行配置 so 优化方案会消耗较多的人力成本，并且有配置无效的可能。为了降低配置成本、加快优化方案的推进速度、保证配置的有效性和正确性，我们在构建平台上统一支持了 so 的优化（支持使用任意构建工具的项目）。业务只需进行简单的配置即可开启 so 的体积优化。



### 配置导出符号的注意事项


注意事项有以下两点：



1. 如果一个 so 的某些符号，被其他 so 通过 dlsym 方式使用，那么这些符号也应该保留在该 so 的导出符号中（否则会导致运行时异常）。
2. 编写 `version_script.txt` 时需要注意 C\+\+ 等语言对符号的修饰，不能直接把函数名填写进去。符号修饰就是把一个函数的命名空间（如果有）、类名（如果有）、参数类型等都添加到最终的符号中，这也是 C\+\+ 语言实现重载的基础。有两种方式可以把 C\+\+ 的函数添加到导出符号中：第一种是查看未优化 so 的导出符号表，找到目标函数被修饰后的符号，然后填写到 `version_script.txt` 中。例如有一个 MyClass 类：


```C++
class MyClass{
   void start(int arg);
   void stop();
};
```


要确定 start 函数真正的符号可以对未优化的 libexample.so 执行以下命令。因为 C\+\+ 对符号修饰后，函数名是符号的一部分，所以可以通过 grep 加快查找：



![图4 查找 start 函数真正符号](https://p1.meituan.net/travelcube/3ee26ad2baa7796c41ae657fbe927bce17894.png)



可以看到 start 函数真正的符号是 `_ZN7MyClass5startEi`。如果想导出该函数，`version_script.txt` 相应位置填入 `_ZN7MyClass5startEi` 即可。



第二种方式是在 `version_script.txt` 中使用 extern 语法，如下所示：



```
{
    global:
      extern "C++" {
      	MyClass::start*;
        "MyClass::stop()";
      };
    local:*;
};
```


上述配置可以导出 MyClass 的 start 和 stop 函数。其原理是，链接时链接器对每个符号进行 demangle（解构，即把修饰后的符号还原为可读的表示），然后与 extern “C\+\+” 中的条目进行匹配，如果能与任一条目匹配成功就保留该符号。匹配的规则是：有双引号的条目不能使用通配符，需要全字符串完全匹配才可以（例如 stop 条目，如果括号之间多一个空格就会匹配失败）。对于没有双引号的条目能够使用通配符（例如 start 条目）。



### 查看优化后 so 的导出符号


业务对 so 进行优化之后，需要查看最终的 so 文件中保留了哪些导出符号，验证优化效果是否符合预期。在 Mac 和 Linux 下均可使用下述命令查看 so 保留了哪些导出符号：



```
nm -D --defined-only xxx.so
```


例如：



![图5 nm命令查看so文件的导出符号](https://p0.meituan.net/travelcube/1762176f232f1b4fcbc86fff624ce3e623091.png)



可以看出，libexample.so 的导出符号有两个：`JNI_OnLoad` 和 `Java_com_example_MainActivity_stringFromJNI`。



### 解析崩溃堆栈


本文的优化方案会移除非必要导出的动态符号，那 so 如果发生崩溃的话是不是就无法解析崩溃堆栈了呢？答案是完全不会影响崩溃堆栈的解析结果。



“so 可优化内容分析”一节已经提过，使用带调试信息和符号表的 so 解析线上崩溃，是分析 so 崩溃的标准方式（这也是 Google 解析 so 崩溃的方式）。本文的优化方案并未修改调试信息和符号表，所以可以使用带调试信息和符号表的 so 对崩溃堆栈进行完整的还原，解析出崩溃堆栈每个栈帧对应的源码文件、行号和函数名等信息。业务编译出 release 版的 so 后将相应的带调试信息和符号表的 so 上传到 crash 平台即可。



## 6. 方案收益


优化 so 对安装包体积和安装后占用的本地存储空间有直接收益，收益大小取决于原 so 冗余代码数量和导出符号数量等具体情况，下面是部分 so 优化前后占用安装包体积的对比：



|so  |优化前大小|优化后大小|优化百分比|
|----|----------|----------|----------|
|A 库|4.49 MB   |3.28 MB   |27.02%    |
|B 库|995.82 KB |728.38 KB |26.86%    |
|C 库|312.05 KB |153.81 KB |50.71%    |
|D 库|505.57 KB |321.75 KB |36.36%    |
|E 库|309.89 KB |157.08 KB |49.31%    |
|F 库|88.59 KB  |62.93 KB  |28.97%    |


下面是上述 so 优化前后占用本地存储空间的对比：



|so  |优化前大小|优化后大小|优化百分比|
|----|----------|----------|----------|
|A 库|10.67 MB  |7.04 MB   |34.05%    |
|B 库|2.35 MB   |1.61 MB   |31.46%    |
|C 库|898.14 KB |386.31 KB |56.99%    |
|D 库|1.30 MB   |771.47 KB |41.88%    |
|E 库|890.13 KB |398.30 KB |55.25%    |
|F 库|230.30 KB |146.06 KB |36.58%    |


## 7. 总结与后续计划


对 so 体积进行优化不仅能够减小安装包体积，而且能获得以下收益：



* 删除了大量的非必要导出符号从而提升了 so 的安全性。
* 因为 `.data``.bss``.text` 等运行时占用内存的 section 减小了，所以也能减小应用运行时的内存占用。
* 如果优化过程中减少了 so 对外依赖的符号，还可以加快 so 的加载速度。


我们对后续工作做了如下的规划：



* 提升编译速度。因为使用 LTO、gc sections 等会增加编译耗时，计划调研 ThinLTO 等方案对编译速度进行优化。
* 详细展示保留各个函数/数据的原因。
* 进一步完善平台优化 so 的能力。


## 8. 参考资料


1. [https://www.cs.cmu.edu/afs/cs/academic/class/15213\-f00/docs/elf.pdf](https://www.cs.cmu.edu/afs/cs/academic/class/15213-f00/docs/elf.pdf)
2. [https://llvm.org/docs/LinkTimeOptimization.html](https://llvm.org/docs/LinkTimeOptimization.html)
3. [https://gcc.gnu.org/onlinedocs/gccint/LTO\-Overview.html](https://gcc.gnu.org/onlinedocs/gccint/LTO-Overview.html)
4. [https://sourceware.org/binutils/docs/ld/VERSION.html](https://sourceware.org/binutils/docs/ld/VERSION.html)
5. [https://clang.llvm.org/docs](https://clang.llvm.org/docs)
6. [https://gcc.gnu.org/onlinedocs/gcc](https://gcc.gnu.org/onlinedocs/gcc)


## 9. 本文作者


洪凯、常强，来自美团平台/App技术部。





