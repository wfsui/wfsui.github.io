---
title: "Spark向量化计算在美团生产环境的实践"
date: 2024-06-26T02:47:04+0000
tags: [美团技术团队, 基础研发平台, 后台, Spark, 向量化计算, Gluten, Velox]
---

## 1 什么是向量化计算


### 1.1 并行数据处理：SIMD指令


让我们从一个简单问题开始：假设要实现“数组a\+b存入c”，设三个整型数组的长度都是100，那么只需将“c\[i\] = a\[i\] \+ b\[i\]”置于一个100次的循环内，代码如下：



```
void addArrays(const int* a, const int* b, int* c, int num) {
  for (int i = 0; i < num; ++i) {
    c[i] = a[i] + b[i];
  }
}
```


我们知道：**计算在CPU内完成，逻辑计算单元操作寄存器中的数据，算术运算的源操作数要先放置到CPU的寄存器中，哪怕简单的内存拷贝也需要过CPU寄存器**。所以，完成“c\[i\] = a\[i\] \+ b\[i\]”需经三步：



1. 加载（Load），从内存加载2个源操作数（a\[i\]和b\[i\]）到2个寄存器。
2. 计算（Compute），执行加法指令，作用于2个寄存器里的源操作数副本，结果产生到目标寄存器。
3. 存储（Store），将目标寄存器的数据存入（拷贝）到目标内存位置（c\[i\]）。


其中，加载和存储对应访存指令（Memory Instruction），计算是算术加指令，循环执行100次上述三步骤，就完成了“数组a \+ 数组b =\> 数组c”。该流程即对应传统的计算架构：单指令单数据（SISD）顺序架构，任意时间点只有一条指令作用于一条数据流。如果有更宽的寄存器（超机器字长，比如256位16字节），一次性从源内存同时加载更多的数据到寄存器，一条指令作用于寄存器x和y，在x和y的每个分量（比如32位4字节）上并行进行加，并将结果存入寄存器z的各对应分量，最后一次性将寄存器z里的内容存入目标内存，那么就能实现单指令并行处理数据的效果，这就是单指令多数据（SIMD）。



![图1：向量化计算“数组a+b存入c”](https://p0.meituan.net/travelcube/28cc423ccab290d315c483bf6461b939110800.png)



单指令多数据对应一类并行架构（现代CPU一般都支持SIMD执行），单条指令同时作用于多条数据流，可成倍的提升单核计算能力。SIMD非常适合计算密集型任务，它能加速的根本原因是“从一次一个跨越到一次一组，从而实现用更少的指令数完成同样的计算任务。”



1996年，Intel推出的X86 MMX（MultiMedia eXtension）指令集扩展可视为SIMD的起点，随后演进出SSE（1999年）SSE2/3/4/5、AVX（2008）/AVX2（2013）、AVX512（2017）等扩展指令集。在linux系统中可以通过lscpu或cpuid命令查询CPU对向量化指令的支持情况。



### 1.2 向量化执行框架：数据局部性与运行时开销


执行引擎常规按行处理的方式，存在以下三个问题：



1. CPU Cache命中率差。一行的多列（字段）数据的内存紧挨在一起，哪怕只对其中的一个字段做操作，其他字段所占的内存也需要加载进来，这会抢占稀缺的Cache资源。Cache命失会导致被请求的数据从内存加载进Cache，等待内存操作完成会导致CPU执行指令暂停（Memory Stall），这会增加延时，还可能浪费内存带宽。
2. 变长字段影响计算效率。假设一行包括int、string、int三列，其中int类型是固定长度，而string是变长的（一般表示为int len \+ bytes content），变长列的存在会导致无法通过行号算offset做快速定位。
3. 虚函数调用带来额外开销。对一行的多列进行处理通常会封装在一个循环里，会抽象出一个类似handle的接口（C\+\+虚函数）用于处理某类型数据，各字段类型会override该handle接口。虚函数的调用多一步查表，且无法被内联，循环内高频调用虚函数的性能影响不可忽视。


![图2：row by row VS blcok by block](https://p0.meituan.net/travelcube/6ebbf37d0e24ac3fbed523e512d40f9d118589.png)



因此，要让向量化计算发挥威力，只使用SIMD指令还不够，还需要对执行框架层面进行改造，变Row By Row为Block By Block：



1. 数据按列组织替代按行组织（在Clickhouse和Doris里叫Block，Velox里叫Vector），这将提高数据局部性。参与计算的列的多行数据会内存紧凑的保存在一起，CPU可以通过预取指令将接下来要处理的数据加载进Cache，从而减少Memory Stall。不参与计算的列的数据不会与被处理的列竞争Cache，这种内存交互的隔离能提高Cache亲和性。
2. 同一列数据在循环里被施加相同的计算，批量迭代将减少函数调用次数，通过模版能减少虚函数调用，降低运行时开销。针对固定长度类型的列很容易被并行处理（通过行号offset到数据），这样的执行框架也有利于让编译器做自动向量化代码生成，显著减少分支，减轻预测失败的惩罚。结合模板，编译器会为每个实参生成特定实例化代码，避免运行时查找虚函数表，并且由于编译器知道了具体的类型信息，可以对模板函数进行内联展开。


![图3：向量化执行框架示例](https://p0.meituan.net/travelcube/967e549c622732b3b0677c0fb1e2be0b439300.png)



### 1.3 如何使用向量化计算


1. 自动向量化（Auto\-Vectorization）。当循环内没有复杂的条件分支，没有数据依赖，只调用简单内联函数时，通过编译选项（如`gcc -ftree-vectorize`、\-O3），编译器可以将顺序执行代码翻译成向量化执行代码。可以通过观察编译hint输出和反汇编确定是否生产了向量化代码。
    * 编译hint输出，编译：`g++ test.cpp -g -O3 -march=native -fopt-info-vec-optimized`，执行后有类似输出“test.cpp:35:21: note: loop vectorized”。
    * 反汇编，`gdb test + (gdb) disassemble /m function_name`，看到一些v打头的指令（例如vmovups、vpaddd、vmovups等）。
2. 使用封装好的函数库，如Intel Intrinsic function、xsimd等。这些软件包中的内置函数实现都使用了SIMD指令进行优化，相当于high level地使用了向量化指令的汇编，详见：[https://www.intel.com/content/www/us/en/docs/intrinsics\-guide/index.html](https://www.intel.com/content/www/us/en/docs/intrinsics-guide/index.html)。
3. 通过asm内嵌向量化汇编，但汇编指令跟CPU架构相关，可移植性差。
4. 编译器暗示：
    * 使用编译指示符（Compiler Directive），如Cilk（MIT开发的用于并行编程的中间层编程语言和库，它扩展了C语言）里的\#pragma simd和OpenMP里的\#pragma omp simd。
    * Compiler Hint。通过\_\_restrict去修饰指针参数，告诉编译器多个指针指向不相同不重叠的内存，让编译器放心大胆的去优化。
5. 如果循环内有复杂的逻辑或条件分支，那么将难以向量化处理。


以下是一个向量化版本数组相加的例子，使用Intel Intrinsic function：



```
#include <immintrin.h> // 包含Intrinsic avx版本函数的头文件

void addArraysAVX(const int* a, const int* b, int* c, int num) {
  assert(num % 8 == 0); // 循环遍历数组，步长为8，因为每个__m256i可以存储8个32位整数
  for (int i = 0; i < num; i += 8) {  
    __m256i v_a = _mm256_load_si256((__m256i*)&a[i]); // 加载数组a的下一个8个整数到向量寄存器
    __m256i v_b = _mm256_load_si256((__m256i*)&b[i]); // 加载数组b的下一个8个整数到向量寄存器
    __m256i v_c = _mm256_add_epi32(v_a, v_b); // 将两个向量相加，结果存放在向量寄存器
    _mm256_store_si256((__m256i*)&c[i], v_c); // 将结果向量存储到数组c的内存
  }
}

int main(int argc, char* argv[]) {
  const int ARRAY_SIZE = 64 * 1024;
  int a[ARRAY_SIZE] __attribute__((aligned(32))); // 按32字节对齐，满足某些向量化指令的内存对齐要求
  int b[ARRAY_SIZE] __attribute__((aligned(32)));
  int c[ARRAY_SIZE] __attribute__((aligned(32)));
  srand(time(0));
  for (int i = 0; i < ARRAY_SIZE; ++i) {
    a[i] = rand(); b[i] = rand(); c[i] = 0; // 对数组a和b赋随机数初始值
  }

  auto start = std::chrono::high_resolution_clock::now();
  addArraysAVX(a, b, c, ARRAY_SIZE);
  auto end = std::chrono::high_resolution_clock::now();
  std::cout << "addArraysAVX took " << std::chrono::duration_cast<std::chrono::microseconds>(end - start).count() << " microseconds." << std::endl;
  return 0;
}
```


addArraysAVX函数中的\_mm256\_load\_si256、\_mm256\_add\_epi32、\_mm256\_store\_si256都是Intrinsic库函数，内置函数命名方式是



* 操作浮点数：\_mm\(xxx\)\_name\_PT
* 操作整型：\_mm\(xxx\)\_name\_epUY


其中\(xxx\)代表数据的位数，xxx为SIMD寄存器的位数，若为128位则省略，AVX提供的\_\_m256为256位；name为函数的名字，表示功能；浮点内置函数的后缀是PT，其中P代表的是对矢量（Packed Data Vector）还是对标量（scalar）进行操作，T代表浮点数的类型（若为s则为单精度浮点型，若为d则为双精度浮点）；整型内置函数的后缀是epUY，U表示整数的类型（若为无符号类型则为u，否在为i），而Y为操作的数据类型的位数。



**编译**：g\+\+ test.cpp \-O0 \-std=c\+\+11 \-mavx2 \-o test。选项\-O0用于禁用优化（因为开启优化后有可能自动向量化），\-mavx2用于启用AVX2指令集。



**测试发现**：非向量化版本addArrays耗时170微秒，而使用Intrinsic函数的向量化版本addArraysAVX耗时58微秒，耗时降为原来的1/3。



## 2 为什么要做Spark向量化计算


从业界发展情况来看，近几年OLAP引擎发展迅速，该场景追求极致的查询速度，向量化技术在Clickhouse、Doris等Native引擎中得到广泛使用，降本增效的趋势也逐渐扩展到数仓生产。2022年6月DataBricks发表论文《Photon\- A Fast Query Engine for Lakehouse Systems》，Photon是DataBricks Runtime中C\+\+实现的向量化执行引擎，相比DBR性能平均提升4倍，并已应用在Databricks商业版上，但没有开源。2021年Meta开源Velox，一个C\+\+实现的向量化执行库。2022 Databricks Data & AI Summit 上，Intel 与Kyligence介绍了合作开源项目Gluten，旨在为Spark SQL提供Native Vectorized Execution。**Gluten\+Velox的组合，使Java栈的Spark也可以像Doris、Clickhouse等Native引擎一样发挥向量化执行的性能优势**。



从美团内部来看，数仓生产有数万规模计算节点，很多业务决策依赖数据及时产出，若应用向量化执行技术，在不升级硬件的情况下，既可获得可观的资源节省，也能加速作业执行，让业务更快看到数据和做出决策。根据Photon和Gluten的公开数据，应用向量化Spark执行效率至少可以提升一倍，我们在物理机上基于TPC\-H测试Gluten\+Velox相Spark 3.0也有1.7倍性能提升。



![图4：Gluten+Velox在TPC-H上的加速比，来自Gluten](https://p0.meituan.net/travelcube/61783373f58902fec5343926a4855b6318690.png)



## 3 Spark向量化计算如何在美团实施落地


### 3.1 整体建设思路


1. 更关注资源节省而不单追求执行加速。Spark在美团主力场景是离线数仓生产，与OLAP场景不同，时间相对不敏感，但资源（内存为主）基数大成本敏感。离线计算历史已久，为充分利用存量服务器，我们不能依赖硬件加速的手段如更多的内存、SSD、高性能网卡。我们评估收益的核心指标是总「memory\*second」降低。
2. 基于C\+\+/Rust等Native语言而非Java进行开发。Java语言也在向量化执行方面做尝试，但JVM语言对底层控制力弱（如无法直接内嵌SIMD汇编），再加上GC等固有缺陷，还远远谈不上成熟，而系统向的语言（C/C\+\+、Rust）则成为挖掘CPU向量化执行潜能的首选。
3. 可插拔、面向多引擎而非绑定Spark。虽然面向不同工作负载的各类大数据引擎层出不穷，但其架构分层则相似，一般包括编程接口、执行计划、调度、物理执行、容错等，尤其执行算子部分有较多可复用实现。如Meta内部主要大数据引擎有Presto和Spark，建设一个跨引擎的执行库，优化同时支持Presto和Spark显然是更好的选择；OLAP引擎向量化计算本身就是标配；流计算引擎出于性能考虑，也可以攒批而非一条条处理数据（mini batch），因此向量化执行也有性能提升空间。我们认为面向不同场景设计的大数据引擎，有可能共用同一个高性能向量化执行库。
4. 使用开源方案而非完全自研。Spark有几百个function和operator，向量化改造的工作量巨大，从性能、完成度、适配成本、是否支持多引擎、社区的活跃度等方面综合考虑，我们最终选择了Gluten\+Velox的方案。
5. 迁移过程对用户透明，保证数据一致。Spark的几百个function和 operator都要通过C\+\+重新实现，同时还涉及Spark、Gluten、Velox版本变化，很容易实现出现偏差导致计算结果不一致的情况。我们开发了一个用于升级验证的黑盒测试工具（ETL Blackbox Test），可以将一个作业运行在不同版本的执行引擎上进行端到端验证，包括执行时间、内存及CPU资源使用情况、作业数据的对比结果（通过对比两次执行的行数，以及每一列所有数据md5的加和值来确定数据是否一致）。


### 3.2 Spark\+Gluten\+Velox计算流程


通过Spark的plugin功能，Gluten将Spark和向量化执行引擎（Native backend，如Velox）连接起来，分为Driver plugin和Executor Plugin。在Driver端，SparkContext初始化时，Gluten的一系列规则（如ColumnarOverrideRules）通过Spark Extensions注入，这些规则会对Spark的执行计划进行校验，并把Gluten支持的算子转换成向量化算子（如FileScan会转换成NativeFileScan），不能转换的算子上报Fallback的原因，并在回退的部分插入Column2Row、Row2Column算子，生成Substrait执行计划。在Executor端，接收到Driver侧的LaunchTask RPC消息传输的Substrait执行计划后，再转换成Native backend的执行计划，最终通过JNI调用Native backend执行。



Gluten希望能尽可能多的复用原有的Spark逻辑，只是把计算部分转到性能更高的向量化算子上，如作业提交、SQL解析、执行计划的生成及优化、资源申请、任务调度等行为都还由Spark控制。



![图5：Spark+Gluten+Velox架构图](https://p0.meituan.net/travelcube/1df890f023fb32528670ad8f96d2da5b172509.png)



### 3.3 阶段划分


在我们开始Spark向量化项目时，开源版本的Gluten和Velox还没有在业界Spark生产环境大规模实践过，为了降低风险最小代价验证可行性，我们把落地过程分为以下五个阶段逐步进行：



1. **软硬件适配情况确认**。Velox要求CPU支持bmi、bmi2、f16c、avx、avx2、sse指令集，需要先确定服务器是否支持；在生产环境运行TPC\-DS或者TPC\-H测试，验证理论收益；公司内部版本适配，编译运行，跑通典型任务。当时Gluten只支持Spark3.2和Spark3.3，考虑到Spark版本升级成本更高，我们暂时将相关patch反打到Spark3.0上。这个阶段我们解决了大量编译失败问题，建议用社区推荐的OS，在容器中编译&运行；如果要在物理机上运行，需要把相关依赖部署到各个节点，或者使用静态链接的方式（开启vcpkg）。


```
cat /proc/cpuinfo | grep --color -wE "bmi|bmi2|f16c|avx|avx2|sse"
```


1. **稳定性验证**。确定测试集，完善稳定运行需要的feature，以达到比较高的测试通过率，包括支持ORC、Remote shuffle、HDFS适配、堆内堆外的内存配置等。本阶段将测试通过率从不足30%提升到90%左右。
2. **性能收益验证**。由于向量化版本和原生Spark分别使用堆外内存和堆内内存，引入翻倍内存的配置，以及一些高性能feature支持不完善，一开始生产环境测试性能结果不及预期。我们逐个分析解决问题，包括参数对齐、去掉arrow中间数据转换、shuffle batch fetch、Native HDFS客户端优化、使用jemelloc、算子优化、内存配置优化、HBO适配等。本阶段将平均资源节省从\-70%提升到40%以上。
3. **一致性验证**。主要是问题修复，对所有非SLA作业进行大规模测试，筛选出稳定运行、数据完全一致、有正收益的作业。
4. **灰度上线**。将向量化执行环境发布到所有服务器，对符合条件的作业分批上线，增加监控报表，收集收益，对性能不及预期、发生数据不一致的作业及时回退原生Spark上。此过程用户无感知。


整个实施过程中，我们通过收益转化漏斗找到收益最大的优化点，指导项目迭代。下图为2023年某一时期的相邻转化情况。



![图6：Spark向量化项目收益转化漏斗图](https://p0.meituan.net/travelcube/d1099bf54e8d7011c3084495e6b6c3da62968.png)



## 4 美团Spark向量化计算遇到的挑战


### 4.1 稳定性问题


1. **聚合时Shuffle阶段OOM**。在Spark中，Aggregation一般包括Partial Aggregation、Shuffle、Final Aggregation三个阶段，Partial Aggregation在Mapper端预聚合以降低Shuffle数据量，加速聚合过程、避免数据倾斜。Aggregation需要维护中间状态，如果Partial Aggregation占用的内存超过一定阈值，就会提前触发Flush同时后续输入数据跳过此阶段，直接进入ShuffleWrite流程。Gluten使用Velox默认配置的Flush内存阈值（Spark堆外内存\*75%），由于Velox里Spill功能还不够完善（Partial Aggregation不支持Spill），这样大作业场景，后续ShuffleWrite流程可能没有足够的内存可以使用\(可用内存\<25%\*Spark堆外内存\)，会引起作业OOM。我们采用的策略是通过在Gluten侧调低Velox Partial Aggregation的Flush阈值，来降低Partial Aggregation阶段的内存占用，避免大作业OOM。这个方案在可以让大作业运行通过，但是理论上提前触发Partial Aggergation Flush会降低Partial Aggretation的效果。更合理的方案是Partial Aggregation支持Spill功能，Gluten和Velox社区也一直在完善对向量化算子Spill功能的支持。
2. **SIMD指令crash**。Velox对数据复制做了优化，如果该类型对象是128bit（比如LongDecimal类型），会通过SIMD指令用于数据复制以提升性能。如下图所示，Velox库的FlatVector\<T\>::copyValuesAndNulls\(\)函数里的一行赋值会调用T::operator=\(\)，调用的movaps指令必须作用于16B对齐的地址，不满足对齐要求会crash。我们在测试中复现了crash，通过日志确定有未按16B对齐的地址出现。无论是Velox内存池还是Gluten内存池分配内存都强制做了16B对齐，最终确认是Arrow内存池分配出的地址没对齐（Gluten用了三方库Arrow）。这个问题可以通过为LongDecimal类型重载operator=操作符修复，但这样做可能影响性能，也不彻底，因为不能排除还有其他128bit类型对象存在。最终我们与Gluten社区修改了Arrow内存分配策略，强制16B对齐。


![图7：Crash代码示例](https://p0.meituan.net/travelcube/2e3d6831aa83af0f6270654e9713f205129314.png)



### 4.2 支持ORC并优化读写性能


Velox的DWIO模块原生只支持DWRF和Parquet两种数据格式，美团大部分表都是基于ORC格式进行存储的。DWRF文件格式是Meta内部所采用的ORC分支版本，其文件结构与ORC相似，比如针对ORC文件的不同区域，可通过复用DWRF的Reader来完成相关数据内容的读取。



![图8：Dwrf文件格式](https://p0.meituan.net/travelcube/bc9d3c90e0546ab9c9a0f26591d9f128439225.png)



* DwrfReader：用于读取文件层面的元数据信息，包括PostScript、Footer和Header。
* DwrfRowReader：用来读取StripeFooter，以便确定每个column的Stream位置。
* FormatData：用来读取StripeIndex，从而确定每个RowGroup的位置区间。
* ColumnReader：用来读取StripeData，完成不同column的数据抽取。


我们完善了Velox ORC格式的支持，并对读取链路做了优化，主要工作包括：



1. 支持RLEv2解码（Velox\-5443）并在解码过程中完成Filter下推（Velox\-6647）。我们将Apache RLEv2解码逻辑移植到了Velox，通过BMI2指令集来加速varint解码过程中的位运算，并在解码过程中下推过滤不必要的数据。
2. 支持Decimal数据类型（Velox\-5837）以及该数据类型的Filter下推处理（Velox\-6240）。
3. ORC文件句柄复用以降低HDFS的NN处理压力（Velox\-6140）。出于线程安全层面的考虑，HdfsReadFile每次pread都会开启一个新文件句柄来做seek\+read，客户端会向NameNode发送大量open请求，加重HDFS的压力。我们通过将文件的读取句柄在内部做复用处理（thread\_local模式），减少向NN发送的open请求。
4. 使用ISA\-L加速ORC文件解压缩。我们对ORC文件读取耗时trace分析得出，zlib解压缩占总耗时60%，解码占30%，IO和其他仅占10%，解压效率对ORC文件读取性能很关键。为此，我们对ZlibDecompressor做了重构，引入Intel的解压缩向量化库ISA\-L来加速解压缩过程。


基于这些优化，改造后的Velox版ORC Reader读取时间减少一半，性能提升一倍。



![图9：Apache ORC与改造后的Velox ORC读取性能对比，上为Apache ORC](https://p0.meituan.net/travelcube/6c0b4853edb67a8021945276898ec9a486900.png)



### 4.3 Native HDFS客户端优化


首先介绍一下HDFS C\+\+客户端对ORC文件读取某一列数据的过程。第一步，读取文件的最后一个字节来确定PostScript长度，读取PostScript内容；第二步，通过PostScript确定Footer的存储位置，读取Footer内容；第三步，通过Footer确定每个Stripe的元数据信息，读取StripeFooter；第四步，通过StripeFooter确定每个Column的Stream位置，读取需要的Stream数据。



![图10：ORC文件读取过程](https://p0.meituan.net/travelcube/af27dba048624e7e0f16b81f6a14126373177.png)



在生产环境测试中，我们定位到两个数据读取相关的性能问题：



1. **小数据量随机读放大**。客户端想要读取\[offset ~ readEnd\]区间内的数据，发送给DN的实际读取区间却是\[offset ~ endOfCurBlock\]，导致\[readEnd ~ endOfCurBlock\]这部分数据做了无效读取。这样设计主要是为了优化顺序读场景，通过预读来加快后续访问，然而针对随机读场景（小数据量下比较普遍），该方式却适得其反，因为预读出的数据很难被后续使用，增加了读放大行为。我们优化为客户端只向DN传递需要读取的数据区间，DN侧不提前预取，只返回客户端需要的数据。


![图11：读放大过程示意图](https://p0.meituan.net/travelcube/17cc73b1682e07bfb29f4975d11dfef967218.png)



1. **DN慢节点导致作业运行时间变长**。我们发现很多大作业的HDFS长尾耗时非常高，HDFS的平均read时延只有10ms左右，P99.99时延却达到了6秒，耗时最长的请求甚至达到了5分钟，但在不启用EC场景下，HDFS的每个block会有三副本，完全可以切换到空闲DN访问。为此我们对客户端的读请求链路做了重新的设计与调整，实时监测每个DN的负载情况，基于P99.9分位请求时延判定慢节点，并将读请求路由到负载较低的DN上面。


HDFS Native客户端读优化之后，平均读写延迟降低了2/3，吞吐提升2倍。



### 4.4 Shuffle重构


Gluten在shuffle策略的支持上，没有预留好接口，每新增一种shuffle模式需要较大改动。美团有自研的Shuffle Service，其他公司也可能有自己的Shuffle Service（如Celeborn），为了更好适配多种shuffle模式，我们提议对shuffle接口重新梳理，并主导了此讨论和设计。



Gluten中的shuffle抽象第一层是数据格式（Velox是RowVector，Gluten引入的Arrow是RecordBatch），第二层是分区方式（RoundRobin、SinglePart、Hash、Range），如果要支持新shuffle模式（local、remote），需要实现2\*4\*2=16个writer，将会有大量冗余代码。分区具体实现应该与数据格式和shuffle模式无关，我们用组合模式替代继承模式。另外，我们在shuffle中直接支持了RowVector，避免Velox和Arrow对应数据类型之间的额外转换开销。



![](https://p1.meituan.net/travelcube/07fdb924d99e7be357c6d94bee79a662183360.png)![图12：重构前后shuffle模块UML对比](https://p0.meituan.net/travelcube/f7f7e799ef9e4852abe063a4f81070c2110122.png)



### 4.5 适配HBO


HBO（Historical Based Optimization）是通过作业历史运行过程中资源的实际使用量，来预测作业下一次运行需要的资源并设置资源相关参数的一种优化手段。美团过去在原生Spark上通过调配堆内内存取得了8%左右的内存资源节省。



Gluten主要使用堆外内存（off\-heap），这与原生Spark主要使用堆内内存（on\-heap）不同。初期出于稳定性考虑Gluten和原生Spark的运行参数整体一致，总内存大小相同，Gluten off\-heap 占比75%， on\-heap占比25%。这样配置既会导致内存利用率不高（原生Spark的内存使用率58%，向量化版作业内存使用率 38%），也会使一部分作业on\-heap内存配置偏低，在算子回退时导致任务OOM。



我们把HBO策略推广到堆外内存，向量化计算的内存节省比例从30%提升到40%，由于heap内存配置不合理的OOM问题全部消除。



![图13：HBO流程图](https://p1.meituan.net/travelcube/dff6fa330835336f681d77b8352529e874036.png)



### 4.6 一致性问题


1. **低版本ORC数据丢失**。hive\-0.13之前使用的ORC，Footer信息不包含列名，只有ID用来表示第几列（如Col1, Col2…）。Velox TableScan算子在扫表的时候，如果下推的Filter里包含IsNotNull\(A\)，会根据列名A查找该列数据，由于无法匹配到列名，会误判空文件，导致数据缺失。Spark在生成读ORC表的执行计划时，通过访问HiveMetaStore得到表的Schema信息，并在物理算子FileSourceScanExec中保存了表的Schema信息。Gluten对该算子进行doTransform\(\)转换时，会把表的Schema信息序列化到Substrait的ReadRel里。接下来的Substrait计划转Velox计划阶段，我们把表的Schema信息传给Velox的HiveTableHandle，在构造Velox的DwrfReader时修正ORC文件Footer里的Schema信息（如果Footer的Schema不包含列名，就读取表Schema里的对应列的名称进行赋值），解决了这个问题。
2. **count distinct结果错误**。比如这样一条SQL：`select A, B, count(distinct userId), sum(amt) from t group by 1,2` ，Gluten会把count\(distinct userId\) 变为count\(userId\)，通过把userId加到GroupingKey里来实现distinct语义。具体处理过程如下：


![表1：示例SQL在Spark中的处理步骤](https://p0.meituan.net/travelcube/329ee7d68b80787f9baee9c3abcf9d17137629.png)



在第3步的Intermediate Aggregation中，为了节省内存和加速执行，当Velox的HashAggregate算子满足触发Flush的条件时（HashTable内存占用超过阈值或者聚合效果低于阈值），Velox会标记 partialFull=true，触发Flush操作（计算HashTable里已经缓存数据的Intermediate Result），并且后续输入的数据不再执行Intermediate Aggregation的计算，直接进入第4步的Partial Aggregation。如果后续输入的数据里包含重复的userId，count\(userId\)会因为去重不彻底而结果错误。我们短期的修复方案是禁用Intermediate Aggregation的提前Flush功能，直到所有数据都输入之后再获取该阶段的聚合结果。



这个方案的弊端有两个：1）HashTable的内存占用会变大，需要同时开启HashAggregate算子的Spill功能避免OOM；2）直接修改了Velox的HashAggregate算子内部代码，从Velox自身的角度来看，没有单独针对Distinct相关的聚合做处理，随着后续迭代，可能影响所有用到Intermediate Aggregation的聚合过程。



鉴于此，Gluten社区提供了一个更加均衡的解决方案，针对这类Distinct Aggregation，生成执行计划时，Spark的Partial Merge Aggregation不再生成Intermediate Aggregation，改为生成Final Aggregation（不会提前flush、不使用merge\_sum），同时配合聚合函数的Partial Companion函数来返回Intermediate结果。这样就从执行计划转换策略层面规避这个问题，避免对Velox里Final Aggregation内部代码做过多的改动。



1. **浮点类型转换精度错误**。形如查询`SELECT concat(col2, cast(max(col1) as string)) FROM (VALUES (CAST(5.08 AS FLOAT), 'abc_')) AS tab(col1, col2) group by col2;` 在Spark中返回abc\_5.08，在Gluten中返回abc\_5.079999923706055。浮点数5.08不能用二进制分数精确表达，近似表示成5.0799999237060546875。Velox通过函数`folly::to<std::string>(T val)`来实现float类型到string类型的转换，这个函数底层依赖开源库google::double\-conversion, folly里默认设置了输出宽度参数`DoubleToStringConverter::SHORTEST(可以准确表示double类型的最小宽度)`，转换时经过四舍五入之后返回 5.079999923706055。我们把宽度参数改为`DoubleToStringConverter::SHORTEST_SINGLE(可以准确表示float类型的最小宽度)`，转换时经过四舍五入之后返回 5.08。


## 5 上线效果


我们已上线了2万多ETL作业，平均内存资源节省40%\+，平均执行时间减少13%，证明Gluten\+Velox的向量化计算方案生产可行。向量化计算除了能提高计算效率，也能提高读写数据的效率，如某个作业的Input数据有30TB，过去需要执行7小时，绝大部份时间花在了读数据和解压缩上面。使用向量化引擎后，因为上文提到的ISA\-L解压缩优化，列转行的开销节省，以及HDFS Native客户端优化，执行时间减少到2小时内。



![图14：上线优化效果](https://p0.meituan.net/travelcube/31f775b5c4f8165be863dabebf7d2a3975134.png)



## 6 未来规划


我们已上线向量化计算的Spark任务只是小部分，计划2024年能让绝大部分的SQL任务运行在向量化引擎上。



### 6.1 Spark向量化之后对开源社区的跟进策略


Spark、Gluten、Velox三个社区各有自己考虑和版本发布节奏，从一个社区到多个社区的引擎维护复杂度上升。我们的应对有二，一是计算引擎有不同层次，Spark升级主要考虑功能语义实现、执行计划、资源和task调度，Gluten和Velox的升级主要考虑物理算子性能优化，各取所长；二是尽量减少和社区的差异，公司内部适配只在Spark中实现，公司内的UDF以git submodule形式单独维护。



1. 升级到Spark3.5。Gluten最低支持的Spark版本为3.2，23年我们为了降低验证成本，选择在Spark3.0兼容Gluten，但继续升级迭代成本比较高，在推广之前，应该升级到更新的Spark版本。Spark3.5将会是Gluten社区对Spark3.x上长期支持的稳定版本。高版本Spark也有一些额外收益，我们基于TPC\-H实测，Spark3.5相比Spark3.0，「memory\*second」减少40%，执行时间减少17%，根据之前升级经验，生产环境大约能达到一半效果。
2. 保持Spark版本长期稳定。高版本Spark对Hadoop版本的升级迭代带来比较高适配成本，内部迭代的feature也有比较高的迁移成本，因此我们平均3年才会升级一次Spark版本，更多是将需要的feature合并到内部分支。
3. 快速跟进Gluten/Velox新版本。升级到Spark3.5之后，我们内部Spark版本与Gluten社区的兼容性成本很低，并且向量化相关feature还会持续迭代，预计每半年可升级一次线上版本。


### 6.2 提升向量化覆盖率的策略


1. 扩大向量化算子和UDF范围。我们整理了影响权重最高的几十个算子回退问题与Gluten社区一起解决，对于大量内部UDF，则会探索用大模型来将UDF批量改写为C\+\+版本的向量化实现。
2. 扩大File format支持向量化范围。美团内部有约20%的表为textfile格式，还有接近10%的表使用内部开发的format，只能按行读取也不支持下推，加上行转列都会有额外性能开销，影响最终效果。我们将会把textfile全部转为ORC，为自研format提供C\+\+客户端，进一步提升向量化计算性能。


## 7 致谢


感谢[Intel Gluten](https://github.com/apache/incubator-gluten)合作伙伴高明、周渊、宾伟、韦廷、宏泽、莫芮、飞龙、马榕、镇辉、成成等的大力支持和辛勤付出，也感谢[Gluten](https://gluten.apache.org/)和Velox社区贡献者的开源精神和无私奉献。



## 8 本文作者


luhao、左军、lux、kecookier、cx14、陈皮兔、dju，均来自美团基础研发平台。





