---
title: "目标检测开源框架YOLOv6全面升级，更快更准的2.0版本来啦"
date: 2023-02-18T02:57:52+0000
tags: [美团技术团队, 基础研发平台, 算法, 智能视觉部, 视觉, YOLOv6]
---

9月5日，美团视觉智能部发布了YOLOv6 2.0版本，本次更新对轻量级网络进行了全面升级，量化版模型 YOLOv6\-S 达到了 869 FPS，同时，还推出了综合性能优异的中大型网络（YOLOv6\-M/L），丰富了YOLOv6网络系列。其中，YOLOv6\-M/L 在 COCO 上检测精度（AP）分别达到 49.5%/52.5%，在 T4 卡上推理速度分别可达 233⁄121 FPS（batch size =32）。


GitHub下载地址：[https://github.com/meituan/YOLOv6](https://github.com/meituan/YOLOv6)。欢迎Star收藏，随时取用。



**官方出品详细的Tech Report带你解构YOLOv6**：[YOLOv6: A Single\-Stage Object Detection Framework for Industrial Applications](https://arxiv.org/abs/2209.02976)。



![图1 YOLOv6 各尺寸模型与其他YOLO系列的性能对比图](https://p0.meituan.net/travelcube/15886abdda3d653a7a403667b7dfe7bc174369.png)



注：YOLOv6系列模型均在训练300epoch且不使用预训练模型或额外检测数据集下获得，”‡“表示采用了自蒸馏算法，”∗“表示从官方代码库对发布模型进行重新测评的指标。以上速度指标均在T4 TRT7.2 环境下测试。



![表1 YOLOv6 各尺寸模型与其他YOLO系列的性能对比结果](https://p0.meituan.net/travelcube/a51d1e9f0d159f2b32ac1a703c9615f1352637.png)



注：YOLOv6系列模型均在训练300epoch且不使用预训练模型或额外检测数据集下获得，”‡“表示采用了自蒸馏算法，”∗“表示从官方代码库对发布模型进行重新测评的指标。以上速度指标均在T4 TRT7.2 环境下测试。



本次版本升级，主要有以下更新：



**性能更强的全系列模型**



1. 针对中大型模型（YOLOv6\-M/L），设计了新主干网络 CSPStackRep，它在综合性能上比上一版的 Single Path 结构更具优势。
2. 针对不同网络，系统性地验证了各种最新策略/算法的优劣，综合精度和速度，为每类网络选择合适的方案。同时将模型整体训练时间减少了 50%，极大地提升了模型的训练效率。
3. 引入自蒸馏思想并设计了新的学习策略，大幅提升了 YOLOv6\-M/L 的模型精度。
4. 通过训练时 Early Stop 强数据增强及推理时图像 Resize 优化策略，修复了前期版本中输入尺寸对齐到 640x640 后精度损失的问题，提升了现有模型的实际部署精度。


表 1 展示了 YOLOv6 与当前主流的其他 YOLO 系列算法相比较的实验结果，对比业界其他 YOLO 系列，YOLOv6在所有系列均具有一定的优势：



* YOLOv6\-M 在 COCO val 上 取得了 49.5% 的精度，在 T4 显卡上使用 TRT FP16 batchsize=32 进行推理，可达到 233 FPS 的性能。
* YOLOv6\-L 在 COCO val 上 取得了 52.5% 的精度，在 T4 显卡上使用 TRT FP16 batchsize=32 进行推理，可达到 121 FPS 的性能。
* 同时，YOLOv6\-N/ S 模型在保持同等推理速度情况下，大幅提升了精度指标，训练400 epoch 的条件下，N 网络从 35.0% 提升至 36.3%，S 网络从 43.1% 提升至 43.8%。


**量身定制的量化方案**



本次发布还集成了专门针对 YOLOv6 的量化方案，对重参数化系列模型的量化也有参考意义。该方案借鉴 RepOptimizer \[1\] 在梯度更新时做重参数化，解决了多支路动态范围过大导致难以量化的问题，用 RepOptimizer 训练的 YOLOv6 模型可以直接使用训练后量化（Post\-training Quantization，PTQ），而不产生过大的精度损失。



在这一基础上，我们分析了各层的量化敏感性，将部分敏感层以更高精度运算，进一步提升了模型的精度。另外，我们同时发布了针对 2.0 版本的基于逐通道蒸馏的量化感知训练方案 （Quantization\-aware Training，QAT），并结合图优化，YOLOv6\-S 2.0 版本的量化性能可达到 43.3 mAP 和 869 FPS （batch size=32）。



![表2 YOLOv6-S 量化方案与 PaddleSlim 应用于 YOLO 系列模型的量化效果对比](https://p1.meituan.net/travelcube/ae132369bf7224e27304b2ee66f34866280600.png)



注：以上速度指标均在 T4 TRT8.4 环境下测试。对比方法为 PaddleSlim \[30\] 。



不同之处是 PaddleSlim 使用 YOLOv6\-S 1.0 版本，我们的量化方案应用于 2.0 版本。更详尽的关于量化部署实践的相关内容，近期会在美团技术团队公众号上进行推送，敬请期待。



**完备的开发支持和多平台部署适配**



YOLOv6 支持检测模型训练、评估、预测以及模型量化、蒸馏等全链路开发流程，同时支持 GPU（TensorRT）、CPU（OPENVINO）、ARM（MNN、TNN、NCNN）等不同平台的部署，极大简化工程部署时的适配工作。更详细的教程指引请移步 YOLOv6 Github 仓库 Deployment 的部分。



**相关论文**



\[1\] RepOptimizer:[Re\-parameterizing Your Optimizers rather than Architectures](https://arxiv.org/abs/2205.15242)





