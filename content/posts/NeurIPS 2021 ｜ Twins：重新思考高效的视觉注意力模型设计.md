---
title: "NeurIPS 2021 ｜ Twins：重新思考高效的视觉注意力模型设计"
date: 2022-06-13T03:46:32+0000
tags: [美团技术团队, 基础研发平台, 算法, Twins, 主干网模型, Vision Transformer, NeurIPS]
---

## 导读


Twins \[1\] 是美团和阿德莱德大学合作提出的视觉注意力模型，相关论文已被 NeurIPS 2021 会议接收，代码也已在[GitHub](https://github.com/Meituan-AutoML/Twins)上进行开源。NeurIPS（Conference on Neural Information Processing Systems）是机器学习和计算神经科学相关的学术会议，也是人工智能方向的国际顶级会议。



Twins 提出了两类结构，分别是 Twins\-PCPVT 和 Twins\-SVT：



* Twins\-PCPVT 将金字塔 Transformer 模型 PVT \[2\] 中的固定位置编码（Positional Encoding）更改为团队在 CPVT \[3\] 中提出的条件式位置编码 （Coditional Position Encoding, CPE），从而使得模型具有平移等变性（即输入图像发生平移后，输出同时相应发生变化），可以灵活处理来自不同空间尺度的特征，从而能够广泛应用于图像分割、检测等变长输入的场景。
* Twins\-SVT 提出了空间可分离自注意力机制（Spatially Separable Self\-Attention，SSSA）来对图像特征的空间维度进行分组，分别计算各局部空间的自注意力，再利用全局自注意力机制对其进行融合。这种机制在计算上更高效，性能更优。


Twins 系列模型实现简单，部署友好，在 ImageNet 分类、ADE20K 语义分割、COCO 目标检测等多个经典视觉任务中均取得了业界领先的结果。



## 背景


2020 年 9 月，谷歌的视觉注意力模型 （Vision Transformer， ViT） \[4\] 成功将原本用于自然语言处理的 Transformer \[5\] 应用到视觉的分类任务中。ViT 将一幅输入图像切分为若干个图像块（Patch），并把一个图像块类比为一个文字（Word）作为 Transformer 编码器的输入（如图 1 所示），经过 L 层的编码器处理后使用普通的多层感知机（Multilayer Perceptron, MLP）映射到类别空间。ViT 的模型性能大幅超过了卷积神经网络，此后迅速发展成为了当前视觉领域研究的主要热点。



![图1 视觉注意力模型（ViT）将用于自然语言处理任务的 Transformer 应用于视觉任务（来源：ViT [4]）](https://p1.meituan.net/travelcube/799ca7205dad0fd1ad5e8c8604480625136165.png@750w_80q)



Transformer 编码器中多头注意力（Multi\-head attention）的基本计算方法由下式给定，其中 Q、K、V 分别为 Query（查询）、Key（键）、Value（值） 的缩写，d 为编码维度，softmax 为归一化函数，注意力机制可以理解为对输入按照相关性加权的过程。



![](https://p1.meituan.net/travelcube/a61dcfee46987ccba6c83634d2c19c9216524.png@750w_80q)



原生的视觉注意力模型做主干网络并不能很好地适配目标检测、语义分割等常用的稠密预测任务。此外，相比于卷积神经网络，ViT 计算量通常要更大，推理速度变慢，不利于在实际业务中应用。因此设计更高效的视觉注意力模型，并更好地适配下游任务成为了当下研究的重点。香港大学、商汤联合提出的金字塔视觉注意力模型 PVT \[2\] 借鉴了卷积神经网络中的图像金字塔范式来生成多尺度的特征，这种结构可以和用于稠密任务的现有后端直接结合，支持多种下游任务，如图 2（c）所示。但由于 PVT 使用了静态且定长的位置编码，通过插值方式来适应变长输入，不能针对性根据输入特征来编码，因此性能受到了限制。另外，PVT 沿用了 ViT 的全局自注意力机制，计算量依然较大。



![图2 PVT 将卷积神经网络（a）的金字塔范式迁移到视觉注意力模型（b）得到（c），以适应分类、检测、分割多种任务（来源：PVT <sup>[2]</sup>）](https://p0.meituan.net/travelcube/eebbcf9f58abb9763977218a43101c9f295988.png@750w_80q)



微软亚研院提出的 Swin \[6\] 复用了 PVT 的金字塔结构。在计算自注意力时，使用了对特征进行窗口分组的方法（如图 3 所示），将注意力机制限定在一个个小的窗口（红色格子），而后通过对窗口进行错位使不同组的信息产生交互。这样可以避免计算全局自注意力而减少计算量，其缺点是损失了全局的注意力，同时由于窗口错位产生的信息交互能力相对较弱，一定程度上影响了性能。



![图3 Swin 计算每一个红色格子的局部自注意力，通过不同层间的窗口移位来使各局部注意力之间产生交互（来源：Swin [6]）](https://p0.meituan.net/travelcube/35daedb7476c63158e6ec880d1e483fc303574.png@750w_80q)



### 视觉注意力模型设计的难点


简单总结一下，当前视觉注意力模型设计中需要解决的难点在于：



* **高效率的计算**：缩小和卷积神经网络在运算效率上的差距，促进实际业务应用；
* **灵活的注意力机制**：即能够具备卷积的局部感受野和自注意力的全局感受野能力，兼二者之长；
* **利于下游任务**：支持检测、分割等下游任务，尤其是输入尺度变化的场景。


## Twins 模型设计


从这些难点问题出发，基于对当前视觉注意力模型的细致分析，美团视觉智能部重新思考了自注意力机制的设计思路，提出了针对性的解决方案。首先将 PVT \[2\] 和 CPVT \[4\] 相结合，形成 Twins\-PCPVT 来支持尺度变化场景的下游任务。再从自注意机制的效率和感受野角度出发，设计了兼容局部和全局感受野的新型自注意力，叫做**空间可分离自注意力** （Spatially Separable Self\-Attention，SSSA）， 形成了 Twins\-SVT。



### Twins\-PCPVT


Twins\-PCPVT 通过将 PVT 中的位置编码（和 DeiT \[7\] 一样固定长度、可学习的位置编码）替换为 CPVT \[4\] 中的条件位置编码 （Conditional Positional Encodings，CPE）。生成 CPE 的模块叫做位置编码器（Positional Encoding Generator， PEG），PEG 在 Twins 模型中的具体位置是在每个阶段的第 1 个 Transformer Encoder 之后，如下图 4 所示：



![图4 Twins-PCPVT-S 模型结构，使用了 CPVT 提出的位置编码器（PEG）](https://p0.meituan.net/travelcube/4a70efafd99cf64d20b1f921c9fd2e6897879.png)



#### 条件位置编码


下图 5 展示了团队在 CPVT \[4\] 中提出的条件位置编码器的编码过程。首先将 $N\*d$ 的输入序列转为 $H\*W\*d$ 的输入特征，再用 $F$ 根据输入进行条件式的位置编码，而且输出尺寸和输入特征相同，因此可以转为 $N\*d$ 序列和输入特征进行逐元素的加法融合。



![图5 条件位置编码器（PEG）](https://p0.meituan.net/travelcube/a7025ce575918103b6f4f2f91cfe792e73689.png)



其中，编码函数 $F$ 可以由简单的深度可分离卷积实现或者其他模块实现，PEG 部分的简化代码如下。其中输入 feat\_token 为形状为 $B\*N\*d$ 的张量，$B$ 为 batch，$N$ 为 token 个数，$C$ 为编码维度（同图 5 中 $d$）。将 feat\_token 转化为 $B\*d\*H\*W$ 的张量 cnn\_feat 后，经过深度可分离卷积（PEG）运算，生成和输入 feat\_token 相同形状的张量，即条件式的位置编码。



```Python
class PEG(nn.Module):
    def __init__(self, in_chans, embed_dim):
        super(PEG, self).__init__()
        self.peg = nn.Conv2d(in_chans, embed_dim, 3, 1, 1, bias=True, groups=embed_dim)
        
    def forward(self, feat_token, H, W):
        B, N, C = feat_token.shape
        cnn_feat = feat_token.transpose(1, 2).view(B, C, H, W) 
        x = self.peg(cnn_feat) + cnn_feat
        x = x.flatten(2).transpose(1, 2)
        return x
```


由于条件位置编码 CPE 是根据输入生成，支持可变长输入，使得 Twins 能够灵活处理来自不同空间尺度的特征。另外 PEG 采用卷积实现，因此 Twins 同时保留了其平移等变性，这个性质对于图像任务非常重要，如检测任务中目标发生偏移，检测框需随之偏移。实验表明 Twins\-PCPVT 系列模型在分类和下游任务，尤其是在稠密任务上可以直接获得性能提升。该架构说明 PVT 在仅仅通过 CPVT 的条件位置编码增强后就可以获得很不错的性能，由此说明 PVT 使用的位置编码限制了其性能发挥。



### Twins\-SVT


Twins\-SVT （如下图 6 所示）对全局注意力策略进行了优化改进。全局注意力策略的计算量会随着图像的分辨率成二次方增长，因此如何在不显著损失性能的情况下降低计算量也是一个研究热点。Twins\-SVT 提出新的融合了局部\-全局注意力的机制，可以类比于卷积神经网络中的深度可分离卷积 （Depthwise Separable Convolution），并因此命名为空间可分离自注意力（Spatially Separable Self\-Attention，SSSA）。与深度可分离卷积不同的是，Twins\-SVT 提出的空间可分离自注意力（如下图 7 所示）是对特征的空间维度进行分组，并计算各组内的自注意力，再从全局对分组注意力结果进行融合。



![图6 Twins-SVT-S 模型结构，右侧为两个相邻 Transformer Encoder 的结合方式](https://p0.meituan.net/travelcube/0d1e466c1664cbe5a22349ba1333b86a113849.png)



![图7 Twins 提出的空间可分离自注意力机制 （SSSA）](https://p1.meituan.net/travelcube/7db6cc49ddb247fc21c4a820e56ef7d8320327.png)



空间可分离自注意力采用局部\-全局自注意力（LSA\-GSA）相互交替的机制，分组计算的局部注意力可以高效地传导到全局。LSA 可以大幅降低计算成本，复杂度从输入的平方 $O\(H^2W^2d\)$ 降为线性的 $O\(mnHWd\)$。其中分组局部注意力 LSA 关键实现（初始化函数略）如下：



```Python
class LSA(nn.Module):
    def forward(self, x, H, W):
        B, N, C = x.shape
        h_group, w_group = H // self.ws, W // self.ws # 根据窗口大小计算长（H）和宽（W）维度的分组个数
        total_groups = h_group * w_group
        x = x.reshape(B, h_group, self.ws, w_group, self.ws, C).transpose(2, 3) # 将输入根据窗口进行分组 B* h_group * ws * w_group * ws * C
        qkv = self.qkv(x).reshape(B, total_groups, -1, 3, self.num_heads, C // self.num_heads).permute(3, 0, 1, 4, 2, 5) # 计算各组的 q， k， v 
        q, k, v = qkv[0], qkv[1], qkv[2]
        attn = (q @ k.transpose(-2, -1)) * self.scale # 计算各组的注意力
        attn = attn.softmax(dim=-1) # 注意力归一化
        attn = self.attn_drop(attn) # 注意力 Dropout 层
        attn = (attn @ v).transpose(2, 3).reshape(B, h_group, w_group, self.ws, self.ws, C) # 用各组内的局部自注意力给 v 进行加权
        x = attn.transpose(2, 3).reshape(B, N, C)
        x = self.proj(x) # MLP 层
        x = self.proj_drop(x) # Dropout 层
        return x
```


高效融合 LSA 注意力的 GSA 关键实现（初始化函数略）如下。相比于 ViT 原始的全局自注意力，GSA 的 K、V 是在缩小特征的基础上计算的，但 Q 是全局的，因此注意力仍然可以恢复到全局。这种做法显著减少了计算量。



```Python
class GSA(nn.Module):
    def forward(self, x, H, W):
        B, N, C = x.shape
        q = self.q(x).reshape(B, N, self.num_heads, C // self.num_heads).permute(0, 2, 1, 3) # 根据输入特征 x 计算查询张量 q
        x_ = x.permute(0, 2, 1).reshape(B, C, H, W)
        x_ = self.sr(x_).reshape(B, C, -1).permute(0, 2, 1) # 缩小输入特征的尺寸得到 x_
        x_ = self.norm(x_) # 层归一化 LayerNorm
        kv = self.kv(x_).reshape(B, -1, 2, self.num_heads, C // self.num_heads).permute(2, 0, 3, 1, 4) # 根据缩小尺寸后的特征后 x_，计算 k, v 
        k, v = kv[0], kv[1]
        attn = (q @ k.transpose(-2, -1)) * self.scale # 计算全局自注意力
        attn = attn.softmax(dim=-1)
        attn = self.attn_drop(attn)
        x = (attn @ v).transpose(1, 2).reshape(B, N, C) # 根据全局自注意力对 v 加权
        x = self.proj(x)
        x = self.proj_drop(x)
        return x
```


从上述代码中可以看出，SVT 系列在实现上采用现有主流深度学习框架中的已有操作，不需要额外的底层适配，因此部署起来比较方便。



## 实验


### ImageNet\-1k 分类


Twins\-PCPVT 和 Twins\-SVT 在 ImageNet\-1k 分类任务上，相比同等量级模型均取得 SOTA 结果，吞吐率占优。另外，Twins 支持 TensorRT 部署，Twins\-SVT\-S 模型使用 NVIDIA TensorRT 7.0 推理可以有 1.6 倍的加速，吞吐率可以从 PyTorch 实现的 1059（images/s）提升到 1732。



![表1 ImageNet-1k 分类](https://p1.meituan.net/travelcube/e36b0d6ca89e7cef5abcc9b07aec8f29341916.png)



### ADE20K 分割


在语义分割任务 ADE20K 上，Twins 模型做主干网分别使用 FPN 和 Upernet 后端，相比 PVT 和 Swin 也达到了更好结果，见下表 2：



![表2 ADE20K 分割](https://p0.meituan.net/travelcube/a7eb6197832f77bc5eba5000cf441ff6246969.png)



### COCO 目标检测（RetinaNet 框架）


在经典的 COCO 目标检测任务中，使用 Retina 框架，Twins 模型大幅优于 PVT。而且 Twins\-PCPVT 系列证明 PVT 在通过 CPVT 的编码方式增强之后，可以媲美 Swin 同量级模型，见下表 3：



![表3 COCO 目标检测（Retina 框架）](https://p1.meituan.net/travelcube/3f2f777db764cbc5e6128182732d0cd9252284.png)



### COCO 目标检测（Mask\-RCNN 框架）


在 Mask\-RCNN 框架下，Twins 模型在 COCO 上也有很好的性能优势，且在更长时间训练（3x）时得以保持，见下表 4：



![表4 COCO 目标检测（Mask-RCNN 框架）](https://p0.meituan.net/travelcube/fc1052e3041dbef2f5ea85c657440d25251837.png)



## 在高精地图多要素语义分割场景的应用


高精地图是自动驾驶中的关键组成部分，在美团无人配送、网约车等业务承担着非常重要的作用。道路场景关键要素的语义提取作为高精建图的前序流程，对建图的质量有直接的影响。多要素语义分割是语义提取的重要一环，业界一般采用经典的语义分割算法来实现。



此处，我们以 DeepLab 系列 \[8\] 为代表做介绍，分割模型通常分为编码和解码两个阶段，使用卷积神经网络提取特征，并采用空间金字塔迟化（Spatial Pyramid Pooling），以及不同尺度空洞卷积（Atrous Conv）操作（如下图 8a 所示）来增加全局感受野。这种设计一方面受限于卷积神经网络的特征提取能力，另一方面对全局关系的建模能力有限，导致在分割任务上对细节关注不够，边缘往往不够清晰。



![图8 经典语义分割模型架构（DeepLabV3+ [8]）](https://p0.meituan.net/travelcube/9b88dd87d5dbb64c208d62c3716a70f7469291.png@750w_80q)



Twins 虽然大幅提升了视觉注意力模型的效率和性能，但为了保持和卷积神经网络的接近的推理效率，我们仍需要对模型的后端结构作进一步的优化。不同于论文中为了与其他方法做公平对比而使用的较重的 FPN \[9\] 或 UperNet \[10\] 后端，我们设计了如下图 9 所示的简单轻量的后端，并在业务数据集上的性能和推理速度之间取得了很好的平衡。这个后端是根据 Twins 的特性而设计，由于 Twins 兼顾了全局和局部两种注意力，因此后端无需采用复杂的设计来增大感受野，只通过各尺度特征的线性变化和缩放，就直接恢复到相同尺寸并进行拼接（Concat），简单维度变换后就可以输出分割结果。



![图9 轻量化的 Twins 分割后端设计](https://p1.meituan.net/travelcube/78a0ef2e1213efa21c8201cb0ead82b6382510.png@750w_80q)



从下图 10 的分割结果对比看，Twins 为主干网的模型可以提取得到更精细的图像边缘，如隔离带、道路标牌、路灯杆等关键道路要素和标注真值（Ground Truth）之间的差异更小。



![图10 道路多要素语义提取结果对比](https://p0.meituan.net/travelcube/c9aa8c58c3d9686a49759865509a4e3c285103.png@750w_80q)



## 总结


视觉注意力模型是当前视觉领域的研究重点，并且已经在各类视觉任务上展示了相比经典卷积神经网络的优越性，但在效率上仍需精心优化，在效果上也需要继续提升。探索设计更高效率的注意力模型并促进前沿的视觉研究转向工业落地，对美团业务也具有重要的意义。



此次，美团和阿德莱德大学合作设计的 Twins 系列模型架构，有效降低了计算成本，提升了模型性能，更好地支持了如检测和分割等稠密任务。此外，我们将 Twins 应用在美团高精地图的要素语义分割场景中，带来了更精细的分割结果，提升了高精地图的建图质量。后续，视觉团队将持续探索高效的视觉注意力模型设计，并期望在美团更广泛的业务场景中得到实践和应用。



## 参考文献


* \[1\] [Twins: Revisiting the Design of Spatial Attention in Vision Transformers](https://proceedings.neurips.cc/paper/2021/hash/4e0928de075538c593fbdabb0c5ef2c3-Abstract.html)
* \[2\] [Pyramid Vision Transformer: A Versatile Backbone for Dense Prediction without Convolutions](https://arxiv.org/pdf/2102.12122.pdf)
* \[3\] [Conditional Positional Encodings for Vision Transformers](https://arxiv.org/abs/2102.10882)
* \[4\] [An image is worth 16x16 words: Transformers for image recognition at scale](https://openreview.net/pdf?id=YicbFdNTTy)
* \[5\] [Attention Is All You Need](https://arxiv.org/abs/1706.03762)
* \[6\] [Swin Transformer: Hierarchical Vision Transformer using Shifted Windows](https://arxiv.org/pdf/2103.14030.pdf)
* \[7\] [Training data\-efficient image transformers & distillation through attention](https://arxiv.org/abs/2012.12877)
* \[8\] [Encoder\-Decoder with Atrous Separable Convolution for Semantic Image Segmentation](https://arxiv.org/abs/1802.02611)
* \[9\] [Panoptic Feature Pyramid Networks](https://arxiv.org/abs/1901.02446)
* \[10\] [Unified Perceptual Parsing for Scene Understanding](https://arxiv.org/pdf/1807.10221.pdf)


## 作者简介


视觉智能部祥祥、田值、张勃、晓林，自动车配送部海兵、华夏。



## 团队简介及招聘信息


美团视觉智能部 AutoML 算法团队旨在通过 AutoML 及前沿的视觉技术赋能公司各项业务、加速算法落地，涵盖 AutoML、分割、检测（2D、3D）、Self\-training 等技术方向。欢迎感兴趣的校招和社招同学发送简历至：chuxiangxiang@meituan.com，实习、正式均可。



美团自动车配送部高精地图团队是高精地图技术研发团队，我们的职责是为美团自动驾驶提供高精度、高鲜度、大范围的高精地图服务。高精地图是涉及多种学科的综合技术，不仅需要依托 SLAM、地理测绘、深度学习、多传感器定位等算法进行高精地图的构建，而且还需要利用大数据技术、高性能计算、高并发服务等进行大规模高精地图处理、存储和查询服务。高精地图团队长期招聘计算机视觉、SLAM、系统开发等专家，感兴趣的同学可以将简历发送至：tech@meituan.com（邮件主题：美团高精地图）。





