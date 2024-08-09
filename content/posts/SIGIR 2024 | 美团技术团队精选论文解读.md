---
title: "SIGIR 2024 | 美团技术团队精选论文解读"
date: 2024-08-09T02:51:20+0000
tags: [美团技术团队, 美团技术团队, 顶会论文, SIGIR, 信息检索, ACM]
---

> 本文精选了美团技术团队被SIGIR 2024收录的3篇论文进行解读，第一篇论文围绕如何利用深度学习，来整合广告拍卖和混排；第二篇论文扩展定义了全用户纵向联邦推荐范式，并首次提出基于检索增强的纵向联邦推荐框架ReFer，解决了跨域特征缺失问题；第三篇论文提出了一种新颖的框架——解耦对比超图学习，并应用于下一个兴趣点推荐任务中。这些论文有美团技术团队的独立产出，也有跟高校、科研机构合作的成果。希望能给从事相关研究工作的同学带来一些帮助或启发。


SIGIR的全称为ACM Special Interest Group on Information Retrieval（ACM国际信息检索大会），是中国计算机学会CCF推荐的A类国际学术会议，也是人工智能领域智能信息检索方向最权威的国际会议。根据会议官方统计，这次会议共收到1148篇长文投稿，其中有791篇有效长文投稿，仅有159篇长文被录用，录用率为20.1%。



![](https://p0.meituan.net/travelcube/d1e538c1a57fd2a5659d3fa289c791842757654.png)



## 01 Deep Automated Mechanism Design for Integrating Ad Auction and Allocation in Feed


## 融合信息流广告拍卖与混排的深度自动机制设计


**论文作�**�：Xuejian Li\*（Meituan），Ze Wang\*（Meituan），Bingqi Zhu（Meituan），Fei He（Meituan），Yongkang Wang（Meituan），Xingxing Wang（Meituan）



备注：\*为共同一作。



**论文类型**：Long Paper



**论文下载**：[PDF](https://arxiv.org/pdf/2401.01656)



![](https://p0.meituan.net/travelcube/89a01154063b0ba0a30e62124735ebd7409025.png)



**论文简介**：电子商务平台通常展示一个包含自然结果和广告的有序列表来响应用户的页面请求。这个列表是广告拍卖和混排的结果，直接影响平台的广告收入和总交易额（GMV），其中广告拍卖决定展示哪个广告及其计费，混排决定广告和自然结果的展示顺序。主流做法将广告拍卖和混排分为两个独立阶段，但这存在两个问题导致次优的结果：1）广告拍卖没有考虑外部性，例如实际展示位置和上下文对广告点击率（CTR）的影响；2）混排利用拍卖获胜广告的计费动态决定展示位置，未能维持广告机制的激励兼容性（IC）。



因此，本文提出了一个深度自动机制，整合了广告拍卖和混排，确保在考虑外部性的情况下实现IC和个体理性（IR），同时最大化广告收入和GMV。该机制将候选广告和自然结果的有序列表作为输入，对于每个候选广告，在自然结果有序列表的不同位置插入广告，生成所有候选分配。对于每个候选分配，页面级别模型将整个分配作为输入，输出每个广告和自然结果的预测结果，以建模全局外部性。最后，基于深度神经网络建模的自动拍卖机制选择最优分配并确定计费。该机制同时决定了广告的排名、计费和展示位置，在离线实验和在线A/B测试中，产生的广告收入和GMV高于最先进的基线。



## 02 ReFer: Retrieval\-Enhanced Vertical Federated Recommendation for Full Set User Benefit


## ReFer: 一种面向全用户增益的检索增强式纵向联邦推荐框架


**论文作�**�：Wenjie Li（Tsinghua）, Zhongren Wang（Tsinghua），Jinpeng Wang（Tsinghua）, Shu\-Tao Xia（Tsinghua），Jile Zhu（Meituan），Mingjian Chen（Meituan），Jiangke Fan（Meituan），Jia Cheng（Meituan）， Jun Lei（Meituan）



**论文类型**：Research Track Full Paper



**论文下载**：[PDF](https://still2009.github.io/pdf/ReFer_SIGIR24_v0624.pdf)



![](https://p0.meituan.net/travelcube/08f893525d5707045ff3d7df6ea8f80d250336.png)



**论文简介**：随着跨企业数据流通的需求增长、和数据隐私保护监管日益严格，纵向联邦学习（Vertical Federated Learning，VFL）这一隐私机器学习技术被更多地应用于推荐系统中。然而传统联邦方案忽略了大量非交叉用户数据，不仅降低了训练过程中用户兴趣信息的丰富度，还导致模型只能对数量有限的交叉用户进行预测，极大降低了商业落地的性价比。



为解决这一问题，本论文扩展定义了全用户纵向联邦推荐范式（Fully Vertical Federated Recommendation，FullyVFR），并首次提出基于检索增强的纵向联邦推荐框架ReFer。该框架提出了一种通用的二阶段分布式检索方案及其配套的分布式注意力融合机制，解决了跨域特征缺失问题，缓解了跨用户群的兴趣偏差，显著提高了全体用户在联邦模型上的性能增益。在公共数据集和美团业务数据集上的实验结果均显示，ReFer能在多个任务场景下提升全体用户群的推荐性能。



## 03 Disentangled Contrastive Hypergraph Learning for Next POI Recommendation


## 解耦对比超图学习用于下一兴趣点推荐


**论文作�**�：Yantong Lai （IIE CAS; UCAS），Yijun Su（IIE CAS），Lingwei Wei（IIE CAS）， Tianqi He（Meituan）， Haitao Wang（Meituan），Gaode Chen（IIE CAS; UCAS），Daren Zha（IIE CAS），Qiang Liu（Meituan），Xingxing Wang（Meituan）



备注：IIE CAS全称为Institute of Information Engineering, Chinese Academy of Sciences；UCAS全称为University of Chinese Academy of Sciences



**论文类型**：Research Track Full Paper



**论文下载**：[PDF](https://s3plus.meituan.net/v1/mss_e63d09aec75b41879dcb3069234793ac/file/Lai%20%E7%AD%89%20-%202024%20-%20Disentangled%20Contrastive%20Hypergraph%20Learning%20for%20N.pdf)



![](https://p1.meituan.net/travelcube/1522aa432b0be3a351d4ff813b43f5f83193810.png)



**论文简介**：下一个兴趣点（POI）推荐是一项重要且流行的任务，旨在为用户提供下一个感兴趣的位置建议。现有的大多数基于序列和图神经网络的方法已探索了多种途径来建模用户的访问行为，并取得了较好的性能。然而，目前仍有两个关键问题尚未得到充分关注：1\) 大多数先前的研究忽视了用户偏好会受不同且不断变化的多方面决策因子影响，导致学到的用户表征耦合且次优；2\) 许多现有方法未能充分建模不同用户决策因子之间的重要协同关联，阻碍了捕捉因子间互补推荐增强的能力。



为了解决这些挑战，本文提出了一种新颖的框架——解耦对比超图学习（DCHL），并应用于下一个兴趣点推荐任务中。具体而言，本文设计了一个多视图解耦超图学习组件，分别从协同、转移和地理视图解耦建模用户\-POI交互行为，并针对性设计各视图感知的超图卷积网络学习解耦的POI表征。另外，本文提出了一个自适应融合方法来自动融合多视图用户表征，并采用了跨视图对比学习方法捕捉视图间的协同关联，实现用户表征和POI表征的表示增强。最后，本文在三个真实世界数据集上进行了充分实验，验证了所提方法相较多类别先进基线方法的优越性。



## 美团科研合作


美团科研合作致力于搭建美团技术团队与高校、科研机构、智库的合作桥梁和平台，依托美团丰富的业务场景、数据资源和真实的产业问题，开放创新，汇聚向上的力量，围绕机器人、人工智能、大数据、物联网、无人驾驶、运筹优化等领域，共同探索前沿科技和产业焦点宏观问题，促进产学研合作交流和成果转化，推动优秀人才培养。面向未来，我们期待能与更多高校和科研院所的老师和同学们进行合作。欢迎老师和同学们发送邮件至：meituan.oi@meituan.com。





