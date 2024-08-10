---
title: "大众点评技术部包揽KDD 2024 OAG-Challenge Cup赛道全部3项冠军"
date: 2024-08-10T02:51:19+0000
tags: [美团技术团队, 美团技术团队, 大众点评, KDD, 比赛获奖]
---

ACM SIGKDD （Knowledge Discovery and Data Mining，简称 KDD）是数据挖掘领域的国际顶级会议。KDD Cup比赛是由SIGKDD主办的数据挖掘研究领域的国际顶级赛事，从1997年开始，每年举办一次，是目前数据挖掘领域最有影响力的赛事。


近日，来自大众点评技术部/搜索与内容智能团队组建的BlackPearl队伍，参加了KDD 2024 OAG\-Challenge Cup赛道的[WhoIsWho\-IND](https://www.biendata.xyz/competition/ind_kdd_2024/)、[PST](https://www.biendata.xyz/competition/pst_kdd_2024/)、[AQA](https://www.biendata.xyz/competition/aqa_kdd_2024/)三道赛题，以较大优势包揽了该赛道全部赛题的冠军。



![](https://p0.meituan.net/travelcube/ebb631e6ad044806ff7e167c3878cc94603739.png)



今年，KDD 2024 OAG\-Challenge Cup 的三道赛题，提出的是针对学术数据挖掘领域中的论文同名消歧、论文源头追溯、学术论文检索三个经典难题。团队同学创新性地采用大模型来解决这三个问题，他们基于大模型，提出自反馈增强、嫁接学习等技术，在效果上显著优于其他队伍，在排行榜上均取得较大领先。



在WhoisWho（同名消歧任务）任务中，团队出了基于自反馈增强的迭代式大模型文本聚类方法，该方法构建的大模型文本聚类方案能够有效处理结构化信息并实现端到端直接输出聚类结果。最终以83%的gAUC指标明显超越传统机器学习方案，赢得了赛题冠军。



![图1 WhoisWho  Solution by BlackPearl](https://p0.meituan.net/travelcube/f8dc212132d3c525e74c2f16e2c38090437996.png)



在PST（论文源头追溯）任务中，团队利用嫁接学习的思想将BERT\-Like模型的复杂文本语义匹配能力嫁接到LLM中，提高样本置信度。同时，团队构建了一套基于RAG的自动特征工程链路，缓解了复杂语义文本普通存在的文本多、信息杂、数据脏的问题。在最终评价指标MAP上利用7B单模型效果超出ChatGPT\+RAG方案10%。



![图2 PST  Solution by BlackPearl](https://p1.meituan.net/travelcube/04b1401688fc544b752b52f672c9dbb2248021.png)



在（AQA学术论文问答任务）任务中，带有复杂噪声的数据是该任务的主要难点。GPT4等开源大模型因为噪声问题，在场景文本搜索方面输出结果完全不可用。团队利用LLM For Vector及Boosting技术在文本搜索场景实践，提出集成召回、排序的Boosting LLM For Searching方案，在指标上全面超越基于传统文本嵌入方式的搜索方案，有效将LLM具备的语义理解能力迁移至场景文本搜索任务，解决了学术搜索场景的噪声问题。



![图3 AQA  Solution by BlackPearl](https://p0.meituan.net/travelcube/c8810ce65b3c30ec6fb686988252528e251099.png)



BlackPearl团队开源了全部解决方案代码供研究者交流学习\([https://github.com/BlackPearl\-Lab/KddCup\-2024\-OAG\-Challenge\-1st\-Solutions\)，并提交了解决方案的论文，团队成员将在巴塞罗那举行的](https://github.com/BlackPearl-Lab/KddCup-2024-OAG-Challenge-1st-Solutions)，并提交了解决方案的论文，团队成员将在巴塞罗那举行的) KDD 2024 上展示报告其研究成果。后续，美团技术团队公众号将陆续发布这三道赛题的技术解读，敬请期待。



未来，大众点评App将不断深入探索大模型技术，充分挖掘其内在潜力，通过先进AI技术产品化，使大众点评能够更精准地服务于用户，努力让AI帮大家更懂美食，更会生活。





