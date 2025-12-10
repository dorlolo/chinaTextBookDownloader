# 贡献指南

感谢您对教材下载器项目的关注！我们欢迎各种形式的贡献。

## Git工作流

本项目采用标准的Git工作流：

### 1. Fork仓库

首先fork本仓库到您的个人账户下。

### 2. 克隆仓库

```bash
git clone https://github.com/dorlolo/chinaTextBookDownloader.git
cd downloader
```

### 3. 创建功能分支

为您的功能或修复创建一个新的分支：

```bash
git checkout -b feature/your-feature-name
# 或者
git checkout -b fix/your-bug-fix
```

### 4. 进行修改

进行您的代码修改，并确保：

- 遵循现有的代码风格
- 添加适当的注释
- 编写必要的测试

### 5. 提交更改

```bash
git add .
git commit -m "Add a brief description of your changes"
```

### 6. 推送到您的fork

```bash
git push origin feature/your-feature-name
```

### 7. 创建Pull Request

在GitHub上创建一个Pull Request，描述您的更改和改进。

## 代码规范

- 使用Go标准格式化工具：`go fmt`
- 保持代码简洁明了
- 添加适当的注释，特别是对于复杂的逻辑
- 遵循Go的命名约定

## 构建和测试

在提交之前，请确保您的代码可以通过构建和测试：

```bash
# 构建项目
make build
# 或者
go build .

# 运行测试
make test
# 或者
go test ./...
```

## 报告问题

如果您发现了bug或有任何建议，请在GitHub Issues中报告。

## 许可证

通过贡献代码，您同意您的贡献将遵循项目的MIT许可证。