---
title: "美团开放平台SDK自动生成技术与实践"
date: 2023-03-09T03:06:35+0000
tags: [美团技术团队, 到店, 系统, 餐饮SaaS, 开放平台, SDK, 代码生成]
---

## 1. 引言


[美团开放平台](http://developer.meituan.com)对外提供了外卖、团购、配送等20余个业务场景的OpenAPI，供第三方开发者搭建应用时使用，是美团系统与外部系统通讯的最重要平台。本文主要讲述开放平台如何通过技术手段自动生成支持接口参数富模型和多种编程语言的SDK，以提高开发者对接开放平台API的效率。



![美团开放平台架构](https://p0.meituan.net/travelcube/bbf283dc58fb7358b62551524cafebd7264179.png)



### 1.1 背景


美团开放平台将美团各类业务提供的扩展服务封装成一系列应用程序编程接口（API）对外开放，供第三方开发者使用。开发者可通过调用开放平台提供的OpenAPI获取数据和能力，以实现自身系统与美团系统协同工作的业务逻辑。以外卖业务场景为例，开发者可以在自己为外卖商户开发的应用中通过调用美团开放平台提供的API，提供外卖订单查询、接单、订单管理等一系列功能。如下图所示：



![](https://p0.meituan.net/travelcube/42fc2d55ccbed47e618eb1f16a94a5e7121354.png)



开放平台为开发者提供的OpenAPI以HTTP接口的形式提供。以平台提供的订单查询接口为例，对应的HTTP请求如下所示：



```http
POST https://api-open-cater.meituan.com/api/order/queryById
Content-Type: application/x-www-form-urlencoded;charset=utf-8

appAuthToken=eeee860a3d2a8b73cfb6604b136d6734283510c4e92282&
charset=utf-8&
developerId=106158&
sign=4656285a4c2493e279d929b8b9f4e29310da8b2b&
timestamp=1618543567&
biz={"orderId": "10046789912119"}

Response:{
  "orderId":"10046789912119",
  "payAmount":"45.67",
  "status":7,
  ......,
  "products":[{"pid":"8213","num":2,...,"price":"3.67"}{"pid":"6556","num":1,...,"price":"11.99"}]
}
```


由上述示例可以看出，美团开放平台提供给开发者的接口契约较为复杂，其中包含了业务规则复杂及安全性要求高等原因。若开发者需要直接从0到1编码对接平台提供的HTTP API，需要关注通信协议、接口契约规范、认证标识传递和安全签名等细节，成本较高。随着业务的发展，平台支持的OpenAPI数量在近两年增长约一倍，达到近1000个，平台运营和研发人员需要投入越来越多的精力去帮助开发者解决接口对接过程中的疑难问题。因此，提供SDK以帮助开发者提高开发对接效率，变得十分有必要。



### 1.2 SDK目标概述


SDK，英文名称为 Software Development Kit，即软件开发工具包，广义上指辅助开发某一类软件的相关工具、文档和范例的集合。在开放平台的场景，我们为开发者提供的SDK应能为其屏蔽调用OpenAPI的通信协议、参数传递规范、接口基础契约（如时间戳、安全签名）等细节，以降低其对接平台API所需的开发成本。具备基本功能的开放平台SDK的架构和功能模块如下所示：



![](https://p0.meituan.net/travelcube/a68a0c26f4d404ed8b2745f4ad21687f193061.png)



从使用SDK的开发者角度来看，基于SDK封装的基础功能来编写调用开放平台接口的代码，大致逻辑如下所示：



```java
MeituanClient client = DefaultMeituanClient.builder(developerId, signKey).build();
//设置请求参数
MeituanRequest request = new MeituanRequest("/api/order/queryById");
request.setParam("orderId","10046789912119");
MeituanResponse response = client.invokeApi(req);
if(response.isSuccess()) {
  long price = (long)response.getField("price");
  String phone = response.getField("customerPhone");
  int orderStatus = (int)response.getField("status");
  //完成业务逻辑
} else {
  log.warn("query order failed with response={}", response);
  //处理接口调用失败的逻辑
}
```


从上述代码可以看出，提供基础功能的SDK已经能够为使用者提供较大的便利。相比从零开始编码对接OpenAPI，使用SDK可以帮助开发者省去处理通信协议、公共参数放置、安全签名计算和返回状态码解析的工作量。但开发者在编写代码设置API的业务参数字段的环节，仍需对照API文档逐个手工填充字段名并按字段类型赋值，并且在获取API返回的业务字段时也需自主填充字段名并解析数据类型，存在较大的不便且易出错。



为解决此问题，我们需要在SDK的能力上更进一步提供对参数富模型的支持，即为每个API提供模型化封装的请求参数和返回参数结构，让使用SDK的开发者可以更加专注于业务逻辑的开发。



在SDK加入参数富模型的支持后，从使用者的角度来看，需要编写的代码如下所示：



```java
MeituanClient client = DefaultMeituanClient.builder(developerId, signKey).build();
//设置请求参数
QueryOrderRequest request = new QueryOrderRequest();
request.setOrderId("10046789912119");
//调用接口
MeituanResponse<QueryOrderResponse> response = client.invokeApi(req);
//处理接口返回
if(response.isSuccess()) {
  QueryOrderResponse orderResponse = response.getData();
  long price = orderResponse.getPrice();
  String phone = orderResponse.getCustomerPhone();
  int orderStatus = orderResponse.getStatus();
  log.info("query order finish, price={}, orderStatus={}", price, phone, orderStatus);
} else {
  log.warn("query order failed with response={}", response);
  //处理接口调用失败的逻辑
}
```


可以看出，参数富模型功能可以进一步减少开发者使用SDK的复杂度。以Java语言版本为例，QueryOrderRequest和QueryOrderResponse两个富模型类中封装了API的请求参数和返回参数的所有字段名、字段类型和字段校验规则等信息，开发者可简单使用字段的getter和setter方法完成对字段的赋值和取值操作，大幅降低了理解成本和出错可能。



虽然在SDK中支持参数富模型功能，可以有效提高使用者的效率，但也会带来SDK的开发和维护成本增加。如果采用纯人工的方式去开发维护SDK中支持的所有API的参数模型代码，需要投入的开发维护成本与SDK支持的编程语言数量和API数量呈正相关性，其成本公式为：



![](https://p0.meituan.net/travelcube/9fee63491e607acef229e4473b0b6ec145219.png)



从上述公式可以看出，当SDK所需支持的API数量和编程语言数量达到一定数量时，通过纯人工编码去开发和维护SDK的成本会非常高。需要通过技术手段自动生成和测试SDK中的绝大部分代码，以达到在成本可控的前提下，为开发者提供支持多种编程语言版本的富模型SDK的目标。



## 2. SDK自动生成技术详解


### 2.1 整体设计


要为开发者提供一个支持参数富模型功能的OpenAPI SDK，我们需要实现以下主要功能：



1. **通信协议封�**�：让开发者无需关注调用API的通信协议和通信逻辑。
2. **接口基础契约封�**�：让开发者无需关注调用API的参数传递格式、时间戳、安全签名、返回Code码处理等细节。
3. **请求参数模型封�**�：让开发者便捷地设置API请求参数。
4. **返回参数模型封�**�：让开发者便捷地使用API返回的数据。


![](https://p0.meituan.net/travelcube/1e6b571164841c5d20bd3c10cb808ebb167860.png)



其中，通信协议封装和接口基础契约封装是一次性工作，并且其逻辑是相对稳定的。对于SDK所需支持的每一种编程语言，只需投入有限的成本开发一次对应代码逻辑，即可支撑SDK的整个生命周期。而要为平台开放的1000余个API提供支持多种编程语言的参数富模型功能，靠人工编写和维护代码是极其低效的，我们考虑通过代码自动生成技术，对SDK中的参数富模型代码进行自动化生成。



更进一步，在实现了参数富模型代码自动生成后，我们可以通过持续集成（Continious Integration）和持续发布（Continuous Delivery）技术，将SDK的生成、测试和发布流程也尽可能地做到自动化。整体的SDK自动生成流程设计如下图所示：



![](https://p0.meituan.net/travelcube/a0a699329c02ba7d660aa8657ee22ceb271149.png)



实现了以上流程后，即可做到在开放平台的任意API的参数模型发生变化时，由系统自动生成和发布最新版本的SDK供开发者使用。我们将在下文详述如何通过代码自动生成、持续集成和持续发布等技术手段实现上述流程。



### 2.2 自动生成参数模型代码


我们最终的目标是为开放平台的每个OpenAPI，自动生成供SDK使用的请求参数模型代码（Request类）、返回参数模型代码（Response类）和调用示例代码（Example），并且代码自动生成机制要支持SDK适配的多种编程语言。以Java和C\#编程语言为例，我们要生成的目标代码如下图所示：



![](https://p0.meituan.net/travelcube/bc7fe218b2ae7871236e9575383a1f84968929.png)



从上面的示例中可以看出，在请求参数模型（Request类）中需要生成Request Path、鉴权配置、字段强类型定义、字段取值、赋值及校验逻辑等代码。在返回参数模型（Response类）中，需要生成接口返回的各个数据字段的强类型定义、取值逻辑及校验规则。调用示例代码则需要包含请求参数赋值、发起接口调用和处理接口返回数据等相关逻辑。



要达成上述目标，首先需要考虑的是代码自动生成技术的选型，目前业界主流的代码生成技术分为以下几类：



![](https://p0.meituan.net/travelcube/97334b642f45b58613377203af4bdf7796748.png)



1. **基于模版编排生成代码**：最原始最简单也是目前应用最广泛的一种代码生成方式。包括后端MVC框架的Controller、Service、DAO层模式化代码一键生成，还有前端Vue CLI 和Create\-React\-App两款脚手架的代码生成，都属于此类。
2. **基于可视化UI生成代码**：目前市场上运用得很广的一门技术，也被称为代码可视化生成工具。从Eclipse的Web可视化编辑器，到.NET Framework提供的MVC，及Winform界面及控件代码可视化拖拽生成，到汽车行业广泛使用的可视化原型搭建工具（自动生成C代码）都属于此类。在近几年比较火的低代码平台（如aPaaS）中，通过可视化UI生成代码的技术也被大量使用。
3. **基于代码语料生成代码**：基于代码语料生产代码的前提是要有足够的语料，例如伪代码/中间语言/描述性代码模板，再基于一套生成规则去生成目标代码。常见的落地场景包括RPC框架中基于IDL（Interface description language，接口描述语言）自动生成多种编程语言的RPC Client和Service代码，以及IDE插件中的代码自动生成功能（例如Eclipse的[telosys](https://marketplace.eclipse.org/content/telosys-tools)插件可通过DSL生成多种语言代码）。
4. **基于人工智能技术生成代码**：属于比较前沿的技术范畴，多和AI领域的图像识别和机器学习技术结合。现有的一些典型案例包括：微软开发的可将手绘图转化HTML代码的智能化代码生成工具[sketch2code](https://www.microsoft.com/en-us/ai/ai-lab-sketch2code)，基于AI技术自动生成UI逻辑的[teleporthq](https://github.com/teleporthq)。


考虑到开放平台SDK中，需要自动生成的OpenAPI参数富模型代码和调用示例代码均具备相对较强的规则性和模式性，我们选择基于代码语料自动生成代码的技术路线。



基于代码语料自动生成代码需要“语料”\+“规则”两个核心元素，我们可以通过解析API元数据并结合领域专用语言（DSL）作为语料模板，生成代码语料，再基于语料特性为不同的编程语言定制代码生成规则，最终将“语料”\+“规则”输入代码生成器以完成目标代码的生成。整体流程如下图所示：



![](https://p0.meituan.net/travelcube/d98ed501c083ca9b5207c4c052b3abcc340972.png)



在上述流程中，首先关注作为代码语料生成数据源的API元数据，其来源于开放平台实现的零编码API网关底层维护的基础配置。开放平台网关基于API元数据配置化的技术，可做到零编码将业务服务的RPC接口转化为HTTP协议的API进行开放。其基本运行结构如下图所示：



![](https://p0.meituan.net/travelcube/49245d3c18d8f755dbefdc6478f689bf230136.png)



作为驱动开放平台网关运行的核心数据，API元数据中包含了HTTP Method、URL、请求参数、返回参数等信息。在参数信息中，又以树形结构记录了每个参数字段的字段名、字段类型、字段描述、校验规则和示例值。我们以“按订单id查询订单详情”的API为例，其元数据中和SDK生成相关的数据如下所示：



```yaml
APIGroup:waimai
APISubGroup:order
APIName: order_query_by_id
HTTP METHOD: POST
HTTP PATH: /api/order/queryById
Description: 按订单id查询订单详情
Request
  |- orderId LONG NOT_NULL 要查询的订单的id example:1000224201796844308
Response
  |- orderId  LONG NOT_NULL 订单id  example:1000224201796844308
  |- price  LONG NOT_NULL 订单金额（单位为人民币“分”） example:3308
  |- phone  STRING  顾客联系电话   example:"13000000002"
  |- products  ARRAY<Product>  订单商品列表
     |- pid  LONG  商品id   example:"13000000002"
     |- name  String  商品名  example:"珍珠奶茶"
     |- num  INTEGER  商品数量  example:1
     |- price  LONG  商品单价   example:1199
     |- properties  ARRAY<Property>  商品属性列表
        |- name STRING 商品属性名  example:"甜度"
        |- value STRING 商品属性值  example:"七分糖"
     |- remark  STRING  商品备注  example:"请做常温的"
  |- status  INTEGER  订单状态  example:7
```


以上信息足以支撑我们为SDK生成参数富模型和调用示例代码。下一步我们需要开始处理代码语料，并为最终的代码自动化生成做好准备。不同编程语言所需的代码语料有所差异，但同一类编程语言（如Java和C\#都是面向对象的编程语言）大致相同。



以生成Java SDK中的参数富模型代码为例，需要用到的代码语料包含两部分。第一部分为类的基本信息，由元数据解析器在解析API的元数据时生成，其包含的内容和具体生成方式如下表所示：



![](https://p0.meituan.net/travelcube/e2d3a67d186f36e9c5b24e47b056e2cb504544.png)



第二部分为语料模板，我们以DSL（Domain Specific Language）作为中间语言加以描述，如下所示：



```java
<@class className=className metaInfo=javaApiMeta baseClass=baseClass interfaces=interfaces classDesc=classDesc package=packageName importPackages=importPackages>
    <#-- 静态字段   -->
    <#if staticFields?? && (staticFields?size > 0) >
        <#list staticFields as param>
            <@staticField param=param/>
        </#list>
    </#if>
    <#-- 字段   -->
    <#if privateFields?? && (privateFields?size > 0) >
        <#list privateFields as param>
            <@field param=param/>
        </#list>
    </#if>
   <#-- Getter/Setter -->
    <#if privateFields?? && (privateFields?size > 0) >
        <#list privateFields as param>
            <@getterMethod param=param/>
            <@setterMethod param=param/>
        </#list>
    </#if>
    
    <#-- 静态字段Getter -->
    <#if staticFields?? && (staticFields?size > 0) >
        <#list staticFields as param>
            <@getterMethod param=param/>
        </#list>
    </#if>

    <#if javaApiMeta?has_content>
        <@deserializeResponse metaInfo=javaApiMeta/>
        <@serializeToJson metaInfo=javaApiMeta/>
    </#if>

    <#-- toString方法 -->
    <#if privateFields?? && (privateFields?size > 0) >
        <@toString className=className params=privateFields/>
    </#if>
</@class>
```


有了上述的代码语料，我们即可通过语言转换引擎生成Java代码。我们将解析好的API元数据作为输入，执行基于DSL的语言转换引擎。语言转换引擎通过执行宏命令将要生成的代码类的基本信息在DSL语料模板中进行填充，最终得到Java编程语言的目标类及其附属类的代码。以生成Response类代码为例，代码生成的具体执行过程如下图所示：



![](https://p1.meituan.net/travelcube/723c582613c49822aeb8f8a6279f7e59636801.png)



Request和Response类中其余的`getter`方法、`setter`方法、类注解等元素的生成原理和步骤均和以上相同，此处不再赘述。在DSL语料模板中所有的元素处理完成后，我们即可得到供Java编程语言使用的请求参数类和返回参数类的完整代码。



对于其他的编程语言（例如Python），我们使用的API元数据和元数据解析逻辑和Java是一致的，不同点在于DSL语料模板和语言转换引擎。当需要对SDK新增一种编程语言的支持时，我们只需要对目标语言建立DSL语料模板并提供相应的转换逻辑，即可支持该语言的请求参数类和返回参数类的代码自动生成。



### 2.3 自动生成API调用示例代码


通过同样的技术手段，我们还可以自动生成每个OpenAPI的调用示例代码，并将示例代码展示接口文档中供开发者参考。



调用示例代码的生成的逻辑相对参数模型代码更加简单。我们使用API元数据中的类名和字段信息（元数据中也包含了每个字段的examle值，可用于在代码示例中生成字段赋值的逻辑）填入代码语料中，再执行语言转换引擎生成目标代码即可。以Java编程语言为例，用于生成API调用示例代码的DSL语料模板如下所示：



```java
<#setting number_format="computer">
MeituanClient meituanClient = DefaultMeituanClient.builder(10000L, "xxxxx").build();

<#assign reqVarName = className?uncap_first/>
${className} ${reqVarName} = new ${className}();

<#if privateFields?? && (privateFields?size > 0)>
<#list privateFields as field>
${reqVarName}.set${field.fieldName?cap_first}(${field.exampleValue!""});
</#list>
</#if>

<#if javaApiMeta.needAuth>
String appAuthToken = "xxxx";
MeituanResponse<${javaApiMeta.responseClass}> response = meituanClient.invokeApi(request, appAuthToken);
<#else >
MeituanResponse<${javaApiMeta.responseClass}> response = meituanClient.invokeApi(request);
</#if>

if (response.isSuccess()) {
<#if javaApiMeta.responseClass == "Void">
    System.out.println("调用成功");
<#else>
    ${javaApiMeta.responseClass} resp = response.getData();
    System.out.println(resp);
</#if>
} else {
    System.out.println("调用失败");
}
```


在使用API元数据和代码语料模板执行基于DSL的语言转换引擎后，生成的API调用示例代码如下所示：



```java
MeituanClient client = DefaultMeituanClient.builder(developerId, signKey).build();
//设置请求参数
OrderQueryByIdRequest request = new OrderQueryByIdRequest();
request.setOrderId(1000224201796844308L);
//调用接口
MeituanResponse<OrderQueryByIdResponse> response = client.invokeApi(req);
//处理接口返回
if(response.isSuccess()) {
  OrderQueryByIdResponse orderResponse = response.getData();
  System.out.println(orderResponse);
} else {
  System.out.println("调用失败");
}
```


可以看出，我们生成的API调用示例代码可以为开发者呈现出每个请求参数赋值的示例逻辑，可有效降低开发者在对接API时的理解成本。后续我们可以进一步优化DSL语料模板，在示例代码中增加对返回数据结构中各个字段的取值逻辑示范，以进一步降低开发者在处理API返回数据时的理解和开发成本。



### 2.4 持续集成和持续发布


搞定参数富模型代码和调用示例代码的自动生成后，下一步是通过持续集成和持续发布技术，确保开发者在任何时刻均能获取到最新版本的SDK。传统由人工编译、测试和上传发布SDK的模式，开发者得到SDK版本更新的周期短则数周，长则数月。我们的目标是将这个周期缩短到分钟级别：当SDK的基础逻辑和API参数模型有任何变更发生时，通过持续集成和持续发布的能力，在数分钟内将包含此变更的新版本SDK发布给开发者使用。



我们基于美团自研的流水线引擎来驱动SDK的持续集成和持续发布。流水线的执行可以看作是对生成SDK的“原材料”一步步加工，最终交付到线上的过程。先通过下图了解整体流程：



![](https://p1.meituan.net/travelcube/bf5a87af7adc4bc44249c5f003fba930270816.png)



首先我们监听可能导致SDK需要发布的变更，包括通过Binlog机制监听API元数据的变更，以及通过Git Hook机制监听SDK基础逻辑代码仓库Master分支的变更。一旦监听到有变更产生，通过触发器去触发SDK持续集成和发布流水线的运作。



流水线开始运作后，首先执行SDK构建组件，SDK构建组件会并发执行两个操作：



1. 获取SDK基础逻辑代码（人工编写）并完成静态代码检查；
2. 拉取API元数据并自动生成参数富模型代码。


以上两个操作完成后，执行代码合并和代码编译，将结果提交到流水线执行下一个步骤。接下来由自动化测试组件完成对SDK的单元测试和端到端自动化测试，通过后提交到流水线执行下一个步骤。最后由自动发布组件完成SDK的打包、上传、下载链接生成和版本信息生成等一系列操作，并最终将最新版本SDK发布到官网供开发者下载。



## 3. 结语


通过上述能力的建设，我们打通了SDK自动生成的整个环节，以自动化的方式完成代码生成、构建、测试、集成、发布等一系列行为，最终实现了在低人力投入的前提下持续向开发者交付最新版本SDK的目标。



通过最近半年数据的对比，我们可以看出开发者使用SDK后在接口对接环节遇到的疑难问题明显减少。基本达到了我们最初提高开发者接入效率，降低平台研发和运营处理工单成本的目标。



![](https://p0.meituan.net/travelcube/8b3c91611d3a102409c1c2bdd8f15312137102.png)



后续，我们将会计划继续完善SDK的代码自动生成逻辑，并为SDK添加更多编程语言的支持，为接入美团开放平台的开发者提供更好的体验。



## 4. 写在后面


不久前，美团独立申报的智慧生活国家新一代人工智能开放创新平台正式获得中华人民共和国科学技术部（以下简称“科技部”）批复。这是美团[第一个国家级科研平台](https://developer.meituan.com/isv/announcement/detail?dockey=anno-all&id=announcement-1733)。



![](https://p1.meituan.net/travelcube/483b8a0334ac6ad76d5ebccd7dee8ee6700451.png)



国家新一代人工智能开放创新平台被称为“人工智能国家队”，是聚焦人工智能重点细分领域，充分发挥行业领军企业的引领示范作用，有效整合技术资源、产业链资源和金融资源，持续输出人工智能核心研发能力和服务能力的重要创新载体。此前，已有百度、阿里、腾讯等15家公司先后获批建设。本次美团成功申报，标志着美团的科研创新能力获得了国家层面认可，达到“国家队水平”。



## 5. 本文作者


飞宏、照东、宇豪、王鸿等，均来自美团到店事业群/餐饮SaaS事业部。





