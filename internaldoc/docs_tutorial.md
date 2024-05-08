## 说明

本说明文档仅限内部使用，不会带到开源仓库。

本文档旨在介绍如何在本地撰写和生成 Kuscia 开源文档以及查看撰写后的文档效果。

## 先决条件

安装依赖

```bash
cd docs/
pip install -r requirements.txt -i https://artifacts.antgroup-inc.cn/simple/
```

## 撰写文档

### rst文档语法参考

* [中文文档](http://www.pythondoc.com/sphinx/contents.html)
* [英文文档](https://www.sphinx-doc.org/en/master/)

### markdown文档语法参考

* [中文文档](https://markdown.com.cn/basic-syntax//)
* [英文文档](https://www.markdownguide.org/basic-syntax/)

### 撰写 Kuscia 开源文档

在以下目录中撰写相应的文档内容

* docs/getting_started
* docs/reference
* docs/development

## 浏览文档效果

### 浏览本地文档效果

1. 将撰写的文档生成html文件

```bash
cd docs/
make html
```

2. 用浏览器打开docs/_build/html/index.html

### 浏览 Antcode secretflow/kuscia 仓库某个分支文档效果

注意：下面IP地址为内部开发机，因此需要关闭浏览器加速代理功能

1. 在浏览器中使用地址 http://100.83.15.142:8089/kuscia/分支名称 渲染分支文档

2. 步骤1完成文档渲染后，会自动跳转到地址 http://100.83.15.142:8088/html/分支名称

其他疑问：
Q1. 若分支代码文档内容有更新，如何查看更新后的文档效果？
ans: 访问步骤1的地址，会重新根据分支最新的内容渲染文档，看到的内容将是最新的文档内容

Q2. 若分支代码文档没有更新，是否每次都需访问步骤1中的地址？
ans: 不需要，若分支代码没有更新，仅需访问步骤2的地址即可，该地址对应一个静态网页。
