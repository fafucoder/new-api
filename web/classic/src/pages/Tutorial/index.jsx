/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useState, useRef, useContext } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Typography, Toast } from '@douyinfe/semi-ui';
import { IconChevronDown, IconCopy } from '@douyinfe/semi-icons';
import { copy } from '../../helpers';
import { useActualTheme } from '../../context/Theme';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { StatusContext } from '../../context/Status';

const { Text } = Typography;

// 自定义内联代码组件
const InlineCode = ({ children }) => {
  const actualTheme = useActualTheme();
  const isDark = actualTheme === 'dark';
  
  return (
    <code 
      className="px-2 py-0.5 rounded text-sm font-mono"
      style={{
        backgroundColor: isDark ? '#2a2a3e' : '#e8e8e8',
        color: isDark ? '#e4e4e4' : '#333',
      }}
    >
      {children}
    </code>
  );
};

// 自定义代码块组件
const CodeBlock = ({ children }) => {
  const ref = useRef(null);
  const { t } = useTranslation();
  const actualTheme = useActualTheme();

  const handleCopy = async () => {
    if (ref.current) {
      const code = ref.current.textContent || '';
      const success = await copy(code);
      if (success) {
        Toast.success(t('代码已复制到剪贴板'));
      } else {
        Toast.error(t('复制失败，请手动复制'));
      }
    }
  };

  const isDark = actualTheme === 'dark';

  return (
    <div
      style={{
        position: 'relative',
        backgroundColor: isDark ? '#1e1e2e' : '#f5f5f5',
        border: `1px solid ${isDark ? '#333' : '#ddd'}`,
        borderRadius: '6px',
        padding: '12px',
        margin: '12px 0',
        overflow: 'auto',
        fontSize: '14px',
        lineHeight: '1.4',
        boxShadow: 'none',
      }}
    >
      <button
        onClick={handleCopy}
        style={{
          position: 'absolute',
          top: '8px',
          right: '8px',
          padding: '4px',
          backgroundColor: isDark ? '#2a2a3e' : '#e0e0e0',
          borderRadius: '4px',
          border: `1px solid ${isDark ? '#444' : '#ccc'}`,
          cursor: 'pointer',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          opacity: 0,
          transition: 'opacity 0.2s ease',
        }}
        onMouseEnter={(e) => {
          e.target.style.opacity = 1;
        }}
        onMouseLeave={(e) => {
          e.target.style.opacity = 0;
        }}
      >
        <IconCopy size={14} style={{ color: isDark ? '#999' : '#666' }} />
      </button>
      <pre
        ref={ref}
        style={{
          margin: 0,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
          color: isDark ? '#e4e4e4' : '#333',
          fontFamily: 'Consolas, Monaco, "Courier New", monospace',
          borderColor: 'inherit',
          boxShadow: 'none',
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.borderColor = 'inherit';
          e.currentTarget.style.boxShadow = 'none';
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.borderColor = 'inherit';
          e.currentTarget.style.boxShadow = 'none';
        }}
      >
        <code>{children}</code>
      </pre>
    </div>
  );
};

// 自定义折叠面板组件 - 参考截图样式
const CollapsiblePanel = ({ title, children, isOpen, onToggle, index }) => {
  const actualTheme = useActualTheme();
  const isDark = actualTheme === 'dark';

  const getIconStyle = () => {
    const styles = [
      { bg: '#22c55e', color: '#fff' },   // 绿色 - 步骤1
      { bg: '#f59e0b', color: '#fff' },   // 橙色 - 步骤2
      { bg: '#3b82f6', color: '#fff' },   // 蓝色 - 步骤3
      { bg: '#8b5cf6', color: '#fff' },   // 紫色 - 步骤4
      { bg: '#ef4444', color: '#fff' },   // 红色 - 步骤5
      { bg: '#06b6d4', color: '#fff' },   // 青色 - 步骤6+
    ];
    return styles[index % styles.length];
  };

  const iconStyle = getIconStyle();

  return (
    <div 
      className="border rounded-lg overflow-hidden mb-2"
      style={{ 
        backgroundColor: isDark ? '#1e1e2e' : '#ffffff', 
        borderColor: isDark ? '#333' : '#e5e5e5',
      }}
    >
      <button
        onClick={onToggle}
        className="w-full px-4 py-3 flex items-center justify-between transition-colors"
        style={{ 
          textAlign: 'left',
          backgroundColor: 'transparent',
          hover: { backgroundColor: isDark ? '#2a2a3e' : '#f5f5f5' },
        }}
        onMouseEnter={(e) => {
          e.target.style.backgroundColor = isDark ? '#2a2a3e' : '#f5f5f5';
        }}
        onMouseLeave={(e) => {
          e.target.style.backgroundColor = 'transparent';
        }}
      >
        <div className="flex items-center gap-3">
          <div
            className="w-7 h-7 rounded-full flex items-center justify-center text-sm font-bold flex-shrink-0"
            style={{
              backgroundColor: iconStyle.bg,
              color: iconStyle.color,
            }}
          >
            {index + 1}
          </div>
          <Text className="font-medium flex-1 text-left" style={{ color: isDark ? '#e4e4e4' : '#333' }}>
            {title}
          </Text>
        </div>
        <IconChevronDown
          size={16}
          className="flex-shrink-0 transition-transform"
          style={{ 
            color: isDark ? '#999' : '#666',
            transform: isOpen ? 'rotate(180deg)' : 'rotate(0deg)',
          }}
        />
      </button>
      {isOpen && (
        <div className="px-4 pb-4 pt-2" style={{ color: isDark ? '#b0b0b0' : '#666' }}>
          {children}
        </div>
      )}
    </div>
  );
};

const Tutorial = () => {
  const { t } = useTranslation();
  const actualTheme = useActualTheme();
  const isMobile = useIsMobile();
  const [statusState] = useContext(StatusContext);
  const [activeTool, setActiveTool] = useState('claude-code');
  const [activeOS, setActiveOS] = useState('macos');
  const [openPanels, setOpenPanels] = useState([0]);

  const serverAddress = statusState?.status?.server_address || `${window.location.origin}`;

  const tools = [
    { key: 'claude-code', label: t('Claude Code') },
    { key: 'codex', label: t('Codex') },
    { key: 'gemini', label: t('Gemini CLI') },
  ];

  const osOptions = [
    { key: 'windows', label: t('Windows') },
    { key: 'macos', label: t('macOS') },
    { key: 'linux', label: t('Linux / WSL2') },
  ];

  const handlePanelToggle = (index) => {
    setOpenPanels((prev) => {
      if (prev.includes(index)) {
        return prev.filter((i) => i !== index);
      }
      return [...prev, index];
    });
  };

  // 教程数据
  const tutorialData = {
    'claude-code': {
      name: t('Claude Code'),
      windows: [
        {
          title: t('安装 Node.js'),
          content: (
            <div className="space-y-4">
              <p>{t('Claude Code 需要 Node.js 18 或更高版本。')}</p>
              <p className="font-medium">{t('Windows 安装方法：')}</p>
              <p>{t('方法一：官方安装包（推荐）')}</p>
              <CodeBlock>{`# 访问 https://nodejs.org 下载 LTS 版本安装包\n# 双击运行安装程序，按提示完成安装`}</CodeBlock>
              <p>{t('方法二：使用 Scoop 包管理器')}</p>
              <CodeBlock>{`scoop install nodejs-lts`}</CodeBlock>
              <p className="text-yellow-400 text-sm">{t('推荐使用官方安装包，安装过程简单且会自动配置环境变量。')}</p>
              <p>{t('验证安装：')}</p>
              <CodeBlock>{`node --version\nnpm --version`}</CodeBlock>
              <p>{t('如果显示版本号（如 v20.x.x 和 10.x.x），说明安装成功。')}</p>
            </div>
          ),
        },
        {
          title: t('安装 Claude Code'),
          content: (
            <div className="space-y-4">
              <p>{t('打开 PowerShell 或 CMD，运行以下命令：')}</p>
              <CodeBlock>{`# 全局安装 Claude Code\nnpm install -g @anthropic-ai/claude-code`}</CodeBlock>
              <p>{t('提示：')}</p>
              <ul className="ml-4 space-y-1">
                <li>• {t('建议使用 PowerShell 而不是 CMD，功能更强大')}</li>
                <li>• {t('如果遇到权限问题，以管理员身份运行 PowerShell')}</li>
              </ul>
              <p>{t('验证 Claude Code 安装：')}</p>
              <CodeBlock>{`claude --version`}</CodeBlock>
              <p>{t('如果显示版本号，说明 Claude Code 已经成功安装了。')}</p>
            </div>
          ),
        },
        {
          title: t('设置环境变量'),
          content: (
            <div className="space-y-4">
              <p>{t('为了让 Claude Code 连接到中转服务，需要设置两个环境变量：')}</p>
              <p className="font-medium">{t('方法一：在系统环境变量中添加（推荐）')}</p>
              <p>{t('按 Win + R，输入 systempropertiesadvanced 回车，点击「环境变量」，在用户变量中新建：')}</p>
              <p>{t('变量名：')}<InlineCode>ANTHROPIC_BASE_URL</InlineCode></p>
              <p>{t('变量值：')}<InlineCode>{serverAddress}</InlineCode></p>
              <p>{t('变量名：')}<InlineCode>ANTHROPIC_AUTH_TOKEN</InlineCode></p>
              <p>{t('变量值：')}<InlineCode>sk_xxxxxxxxx</InlineCode></p>
              <p className="text-yellow-400 text-sm">{t('添加后点击「确定」保存，然后重新打开终端窗口即可生效。')}</p>
              <p className="font-medium">{t('方法二：PowerShell 永久设置（用户级）')}</p>
              <p>{t('在 PowerShell 中运行以下命令设置用户级环境变量：')}</p>
              <CodeBlock>{`# 设置用户级环境变量（永久生效）\n[System.Environment]::SetEnvironmentVariable("ANTHROPIC_BASE_URL", "${serverAddress}", [System.EnvironmentVariableTarget]::User)\n[System.Environment]::SetEnvironmentVariable("ANTHROPIC_AUTH_TOKEN", "sk_xxxxxxxxx", [System.EnvironmentVariableTarget]::User)`}</CodeBlock>
              <p className="text-yellow-400 text-sm">{t('记得将密钥替换为你的实际 API Key。')}</p>
              <p className="font-medium">{t('VSCode Claude 插件配置')}</p>
              <p>{t('如果使用 VSCode 的 Claude 插件，需要创建配置文件：')}</p>
              <ol className="ml-4 space-y-1">
                <li>{t('打开文件资源管理器，在地址栏输入 %USERPROFILE% 回车，进入用户目录')}</li>
                <li>{t('在该目录下创建文件夹 .claude（注意前面有点号）')}</li>
                <li>{t('在 .claude 文件夹内创建文件 config.json')}</li>
                <li>{t('用记事本打开 config.json，写入以下内容并保存：')}</li>
              </ol>
              <CodeBlock>{`{\n "primaryApiKey": "sk_xxxxxxxxx"\n}`}</CodeBlock>
              <p className="text-yellow-400 text-sm">{t('完整路径为 C:\\Users\\你的用户名\\.claude\\config.json')}</p>
              <p>{t('验证环境变量设置：')}</p>
              <p>{t('在 PowerShell 中验证：')}</p>
              <CodeBlock>{`echo $env:ANTHROPIC_BASE_URL\necho $env:ANTHROPIC_AUTH_TOKEN`}</CodeBlock>
              <p>{t('预期输出示例：')}</p>
              <CodeBlock>{`${serverAddress}\ncr_xxxxxxxxxxxxxxxxxx`}</CodeBlock>
            </div>
          ),
        },
        {
          title: t('开始使用 Claude Code'),
          content: (
            <div className="space-y-4">
              <p>{t('启动 Claude Code：')}</p>
              <CodeBlock>{`claude`}</CodeBlock>
              <p>{t('在特定项目中使用：')}</p>
              <CodeBlock>{`# 进入你的项目目录\ncd C:\\path\\to\\your\\project\n\n# 启动 Claude Code\nclaude`}</CodeBlock>
              <p>{t('常用操作：')}</p>
              <ul className="ml-4 space-y-1">
                <li>• {t('输入 claude 启动交互式对话')}</li>
                <li>• {t('输入 claude "你的问题" 直接提问')}</li>
                <li>• {t('输入 claude -p "分析这段代码" 管道模式')}</li>
                <li>• {t('在对话中输入 /help 查看帮助')}</li>
              </ul>
              <p className="text-green-400">{t('配置完成！如果遇到问题，请查看下方的常见问题解答。')}</p>
            </div>
          ),
        },
        {
          title: t('Windows 常见问题解决'),
          content: (
            <div className="space-y-4">
              <div>
                <p className="font-medium">{t('安装时提示"permission denied"错误')}</p>
                <ul className="ml-4 space-y-1">
                  <li>• {t('以管理员身份运行 PowerShell')}</li>
                  <li>• {t('或者配置 npm 使用用户目录：')}<InlineCode>npm config set prefix %APPDATA%\npm</InlineCode></li>
                </ul>
              </div>
              <div>
                <p className="font-medium">{t('PowerShell 执行策略错误')}</p>
                <p className="ml-4">{t('如果遇到执行策略限制，运行：')}</p>
                <CodeBlock>{`Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser`}</CodeBlock>
              </div>
              <div>
                <p className="font-medium">{t('环境变量设置后不生效')}</p>
                <ul className="ml-4 space-y-1">
                  <li>• {t('重新启动 PowerShell 或 CMD')}</li>
                  <li>• {t('或者注销并重新登录 Windows')}</li>
                  <li>• {t('验证设置：')}<InlineCode>echo $env:ANTHROPIC_BASE_URL</InlineCode></li>
                </ul>
              </div>
              <div>
                <p className="font-medium">{t('连接中转服务失败 / 网络错误')}</p>
                <ul className="ml-4 space-y-1">
                  <li>• {t('确认 ANTHROPIC_BASE_URL 地址正确，末尾不要多加 /')}</li>
                  <li>• {t('确认 API Key 格式正确（以 cr_ 开头）')}</li>
                  <li>• {t('检查网络是否可以访问中转服务地址')}</li>
                  <li>• {t('如果使用代理，确保代理配置正确')}</li>
                </ul>
              </div>
              <div>
                <p className="font-medium">{t('Node.js 版本过低导致安装失败')}</p>
                <p className="ml-4">{t('Claude Code 需要 Node.js 18 或更高版本。检查当前版本：')}</p>
                <CodeBlock>{`node --version`}</CodeBlock>
                <p className="ml-4">{t('如果版本低于 18，请从 https://nodejs.org 下载最新 LTS 版本')}</p>
              </div>
            </div>
          ),
        },
      ],
      macos: [
        {
          title: t('安装 Node.js'),
          content: (
            <div className="space-y-4">
              <p>{t('Claude Code 需要 Node.js 18 或更高版本。')}</p>
              <p>{t('方法一：使用 Homebrew（推荐）')}</p>
              <CodeBlock>{`brew install node`}</CodeBlock>
              <p>{t('方法二：访问')} <a href="https://nodejs.org/" target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:underline">Node.js {t('官网')}</a> {t('下载安装')}</p>
              <p>{t('验证安装：')}</p>
              <CodeBlock>{`node --version\nnpm --version`}</CodeBlock>
              <p>{t('如果显示版本号（如 v20.x.x 和 10.x.x），说明安装成功。')}</p>
            </div>
          ),
        },
        {
          title: t('安装 Claude Code'),
          content: (
            <div className="space-y-4">
              <p>{t('打开终端，执行以下命令：')}</p>
              <CodeBlock>{`# 全局安装 Claude Code\nnpm install -g @anthropic-ai/claude-code`}</CodeBlock>
              <p>{t('验证 Claude Code 安装：')}</p>
              <CodeBlock>{`claude --version`}</CodeBlock>
              <p>{t('如果显示版本号，说明 Claude Code 已经成功安装了。')}</p>
            </div>
          ),
        },
        {
          title: t('设置环境变量'),
          content: (
            <div className="space-y-4">
              <p>{t('根据你使用的 shell 配置：')}</p>
              <p className="font-medium">{t('Bash 用户：')}</p>
              <CodeBlock>{`echo 'export ANTHROPIC_BASE_URL="${serverAddress}"' >> ~/.bashrc\necho 'export ANTHROPIC_AUTH_TOKEN="sk_xxxxxxxxx"' >> ~/.bashrc\nsource ~/.bashrc`}</CodeBlock>
              <p className="font-medium">{t('Zsh 用户：')}</p>
              <CodeBlock>{`echo 'export ANTHROPIC_BASE_URL="${serverAddress}"' >> ~/.zshrc\necho 'export ANTHROPIC_AUTH_TOKEN="sk_xxxxxxxxx"' >> ~/.zshrc\nsource ~/.zshrc`}</CodeBlock>
              <p className="text-yellow-400 text-sm">{t('记得将密钥替换为你的实际 API Key。')}</p>
              <p>{t('验证环境变量设置：')}</p>
              <CodeBlock>{`echo $ANTHROPIC_BASE_URL\necho $ANTHROPIC_AUTH_TOKEN`}</CodeBlock>
            </div>
          ),
        },
        {
          title: t('开始使用 Claude Code'),
          content: (
            <div className="space-y-4">
              <p>{t('启动 Claude Code：')}</p>
              <CodeBlock>{`claude`}</CodeBlock>
              <p>{t('在特定项目中使用：')}</p>
              <CodeBlock>{`# 进入你的项目目录\ncd /path/to/your/project\n\n# 启动 Claude Code\nclaude`}</CodeBlock>
              <p>{t('常用操作：')}</p>
              <ul className="ml-4 space-y-1">
                <li>• {t('输入 claude 启动交互式对话')}</li>
                <li>• {t('输入 claude "你的问题" 直接提问')}</li>
                <li>• {t('输入 claude -p "分析这段代码" 管道模式')}</li>
                <li>• {t('在对话中输入 /help 查看帮助')}</li>
              </ul>
              <p className="text-green-400">{t('配置完成！如果遇到问题，请查看下方的常见问题解答。')}</p>
            </div>
          ),
        },
        {
          title: t('macOS 常见问题解决'),
          content: (
            <div className="space-y-4">
              <div>
                <p className="font-medium">{t('安装时提示"permission denied"错误')}</p>
                <ul className="ml-4 space-y-1">
                  <li>• {t('使用 sudo 权限安装：')}<InlineCode>sudo npm install -g @anthropic-ai/claude-code</InlineCode></li>
                  <li>• {t('或者配置 npm 使用用户目录：')}<InlineCode>npm config set prefix ~/.npm</InlineCode></li>
                </ul>
              </div>
              <div>
                <p className="font-medium">{t('环境变量设置后不生效')}</p>
                <ul className="ml-4 space-y-1">
                  <li>• {t('重新打开终端窗口')}</li>
                  <li>• {t('或者执行：')}<InlineCode>source ~/.zshrc</InlineCode></li>
                  <li>• {t('验证设置：')}<InlineCode>echo $ANTHROPIC_BASE_URL</InlineCode></li>
                </ul>
              </div>
              <div>
                <p className="font-medium">{t('Claude Code 命令未找到')}</p>
                <p className="ml-4">{t('确保 npm 全局路径已添加到 PATH，执行：')}<InlineCode>export PATH="$PATH:$(npm prefix -g)/bin"</InlineCode></p>
              </div>
              <div>
                <p className="font-medium">{t('连接中转服务失败')}</p>
                <ul className="ml-4 space-y-1">
                  <li>• {t('确认 ANTHROPIC_BASE_URL 地址正确')}</li>
                  <li>• {t('确认 API Key 格式正确（以 cr_ 开头）')}</li>
                  <li>• {t('检查网络连接')}</li>
                </ul>
              </div>
            </div>
          ),
        },
      ],
      linux: [
        {
          title: t('安装 Node.js'),
          content: (
            <div className="space-y-4">
              <p>{t('Claude Code 需要 Node.js 18 或更高版本。')}</p>
              <p>{t('方法一：使用包管理器')}</p>
              <p className="font-medium">{t('Debian/Ubuntu：')}</p>
              <CodeBlock>{`sudo apt update\nsudo apt install nodejs npm`}</CodeBlock>
              <p className="font-medium">{t('CentOS/RHEL：')}</p>
              <CodeBlock>{`sudo dnf install nodejs npm`}</CodeBlock>
              <p>{t('方法二：使用 nvm（推荐）')}</p>
              <CodeBlock>{`curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash\nnvm install --lts`}</CodeBlock>
              <p>{t('验证安装：')}</p>
              <CodeBlock>{`node --version\nnpm --version`}</CodeBlock>
            </div>
          ),
        },
        {
          title: t('安装 Claude Code'),
          content: (
            <div className="space-y-4">
              <p>{t('打开终端，执行以下命令：')}</p>
              <CodeBlock>{`# 全局安装 Claude Code\nnpm install -g @anthropic-ai/claude-code`}</CodeBlock>
              <p>{t('验证安装：')}</p>
              <CodeBlock>{`claude --version`}</CodeBlock>
            </div>
          ),
        },
        {
          title: t('设置环境变量'),
          content: (
            <div className="space-y-4">
              <p>{t('编辑 ~/.bashrc 或 ~/.zshrc 文件：')}</p>
              <CodeBlock>{`nano ~/.bashrc`}</CodeBlock>
              <p>{t('添加以下内容：')}</p>
              <CodeBlock>{`export ANTHROPIC_BASE_URL="${serverAddress}"\nexport ANTHROPIC_AUTH_TOKEN="sk_xxxxxxxxx"`}</CodeBlock>
              <p>{t('保存后执行：')}</p>
              <CodeBlock>{`source ~/.bashrc`}</CodeBlock>
              <p className="text-yellow-400 text-sm">{t('记得将密钥替换为你的实际 API Key。')}</p>
              <p>{t('验证设置：')}</p>
              <CodeBlock>{`echo $ANTHROPIC_BASE_URL\necho $ANTHROPIC_AUTH_TOKEN`}</CodeBlock>
            </div>
          ),
        },
        {
          title: t('开始使用 Claude Code'),
          content: (
            <div className="space-y-4">
              <p>{t('启动 Claude Code：')}</p>
              <CodeBlock>{`claude`}</CodeBlock>
              <p>{t('在特定项目中使用：')}</p>
              <CodeBlock>{`# 进入你的项目目录\ncd /path/to/your/project\n\n# 启动 Claude Code\nclaude`}</CodeBlock>
              <p>{t('常用操作：')}</p>
              <ul className="ml-4 space-y-1">
                <li>• {t('输入 claude 启动交互式对话')}</li>
                <li>• {t('输入 claude "你的问题" 直接提问')}</li>
                <li>• {t('输入 claude -p "分析这段代码" 管道模式')}</li>
                <li>• {t('在对话中输入 /help 查看帮助')}</li>
              </ul>
              <p className="text-green-400">{t('配置完成！')}</p>
            </div>
          ),
        },
        {
          title: t('Linux/WSL2 常见问题解决'),
          content: (
            <div className="space-y-4">
              <div>
                <p className="font-medium">{t('npm 全局安装权限问题')}</p>
                <p className="ml-4">{t('使用 sudo 或者配置 npm 全局目录权限：')}</p>
                <CodeBlock>{`mkdir -p ~/.npm-global\nnpm config set prefix '~/.npm-global'\necho 'export PATH=~/.npm-global/bin:$PATH' >> ~/.bashrc`}</CodeBlock>
              </div>
              <div>
                <p className="font-medium">{t('WSL2 中环境变量问题')}</p>
                <p className="ml-4">{t('确保在 WSL2 内部设置环境变量，而不是 Windows 系统环境变量')}</p>
              </div>
              <div>
                <p className="font-medium">{t('连接中转服务失败')}</p>
                <ul className="ml-4 space-y-1">
                  <li>• {t('确认 ANTHROPIC_BASE_URL 地址正确')}</li>
                  <li>• {t('确认 API Key 格式正确（以 cr_ 开头）')}</li>
                  <li>• {t('检查防火墙设置，确保可以访问外部网络')}</li>
                </ul>
              </div>
            </div>
          ),
        },
      ],
    },
    'codex': {
      name: t('Codex'),
      windows: [
        {
          title: t('安装 Node.js'),
          content: (
            <div className="space-y-4">
              <p>{t('Codex 需要 Node.js 18 或更高版本。')}</p>
              <p>{t('方法一：官方安装包（推荐）')}</p>
              <CodeBlock>{`# 访问 https://nodejs.org 下载 LTS 版本安装包\n# 双击运行安装程序，按提示完成安装`}</CodeBlock>
              <p>{t('方法二：使用 Scoop 包管理器')}</p>
              <CodeBlock>{`scoop install nodejs-lts`}</CodeBlock>
              <p className="text-yellow-400 text-sm">{t('推荐使用官方安装包，安装过程简单且会自动配置环境变量。')}</p>
              <p>{t('验证安装：')}</p>
              <CodeBlock>{`node --version\nnpm --version`}</CodeBlock>
              <p>{t('如果显示版本号（如 v20.x.x 和 10.x.x），说明安装成功。')}</p>
            </div>
          ),
        },
        {
          title: t('配置 Codex'),
          content: (
            <div className="space-y-4">
              <p>{t('为了让 Codex 连接到中转服务，需要设置环境变量：')}</p>
              <p className="font-medium">{t('方法一：在系统环境变量中添加（推荐）')}</p>
              <p>{t('按 Win + R，输入 systempropertiesadvanced 回车，点击「环境变量」，在用户变量中新建：')}</p>
              <p>{t('变量名：')}<InlineCode>OPENAI_API_BASE</InlineCode></p>
              <p>{t('变量值：')}<InlineCode>{serverAddress}</InlineCode></p>
              <p>{t('变量名：')}<InlineCode>OPENAI_API_KEY</InlineCode></p>
              <p>{t('变量值：')}<InlineCode>sk_xxxxxxxxx</InlineCode></p>
              <p className="text-yellow-400 text-sm">{t('添加后点击「确定」保存，然后重新打开终端窗口即可生效。')}</p>
              <p className="font-medium">{t('方法二：PowerShell 永久设置（用户级）')}</p>
              <CodeBlock>{`[System.Environment]::SetEnvironmentVariable("OPENAI_API_BASE", "${serverAddress}", [System.EnvironmentVariableTarget]::User)\n[System.Environment]::SetEnvironmentVariable("OPENAI_API_KEY", "sk_xxxxxxxxx", [System.EnvironmentVariableTarget]::User)`}</CodeBlock>
              <p className="text-yellow-400 text-sm">{t('记得将密钥替换为你的实际 API Key。')}</p>
              <p>{t('验证环境变量设置：')}</p>
              <CodeBlock>{`echo $env:OPENAI_API_BASE\necho $env:OPENAI_API_KEY`}</CodeBlock>
            </div>
          ),
        },
        {
          title: t('开始使用 Codex'),
          content: (
            <div className="space-y-4">
              <div className="bg-amber-900/30 rounded-lg">
                <p className="font-medium text-amber-400">{t('启动 Codex')}</p>
                <CodeBlock>{`codex`}</CodeBlock>
              </div>
              <div className="bg-gray-800/50 rounded-lg">
                <p className="font-medium">{t('在项目中使用')}</p>
                <CodeBlock>{`# 进入你的项目目录\ncd /path/to/project\n\n# 启动 Codex\ncodex`}</CodeBlock>
              </div>
              <p className="text-green-400">{t('🎉 配置完成！')}</p>
            </div>
          ),
        },
        {
          title: t('Windows 常见问题解决'),
          content: (
            <div className="space-y-4">
              <div>
                <p className="font-medium">{t('环境变量设置后不生效')}</p>
                <ul className="ml-4 space-y-1">
                  <li>• {t('重新启动 PowerShell 或 CMD')}</li>
                  <li>• {t('或者注销并重新登录 Windows')}</li>
                  <li>• {t('验证设置：')}<InlineCode>echo $env:OPENAI_API_BASE</InlineCode></li>
                </ul>
              </div>
              <div>
                <p className="font-medium">{t('连接中转服务失败')}</p>
                <ul className="ml-4 space-y-1">
                  <li>• {t('确认 OPENAI_API_BASE 地址正确')}</li>
                  <li>• {t('确认 API Key 格式正确（以 cr_ 开头）')}</li>
                  <li>• {t('检查网络连接')}</li>
                </ul>
              </div>
            </div>
          ),
        },
      ],
      macos: [
        {
          title: t('安装 Node.js'),
          content: (
            <div className="space-y-4">
              <p>{t('Codex 需要 Node.js 18 或更高版本。')}</p>
              <p>{t('方法一：使用 Homebrew（推荐）')}</p>
              <CodeBlock>{`brew install node`}</CodeBlock>
              <p>{t('方法二：访问')} <a href="https://nodejs.org/" target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:underline">Node.js {t('官网')}</a> {t('下载安装')}</p>
              <p>{t('验证安装：')}</p>
              <CodeBlock>{`node --version\nnpm --version`}</CodeBlock>
            </div>
          ),
        },
        {
          title: t('配置 Codex'),
          content: (
            <div className="space-y-4">
              <p>{t('编辑 ~/.zshrc 或 ~/.bashrc：')}</p>
              <CodeBlock>{`echo 'export OPENAI_API_BASE="${serverAddress}"' >> ~/.zshrc\necho 'export OPENAI_API_KEY="sk_xxxxxxxxx"' >> ~/.zshrc\nsource ~/.zshrc`}</CodeBlock>
              <p className="text-yellow-400 text-sm">{t('记得将密钥替换为你的实际 API Key。')}</p>
              <p>{t('验证设置：')}</p>
              <CodeBlock>{`echo $OPENAI_API_BASE\necho $OPENAI_API_KEY`}</CodeBlock>
            </div>
          ),
        },
        {
          title: t('开始使用 Codex'),
          content: (
            <div className="space-y-4">
              <div className="bg-amber-900/30 rounded-lg">
                <p className="font-medium text-amber-400">{t('启动 Codex')}</p>
                <CodeBlock>{`codex`}</CodeBlock>
              </div>
              <div className="bg-gray-800/50 rounded-lg">
                <p className="font-medium">{t('在项目中使用')}</p>
                <CodeBlock>{`# 进入你的项目目录\ncd /path/to/project\n\n# 启动 Codex\ncodex`}</CodeBlock>
              </div>
              <p className="text-green-400">{t('🎉 配置完成！')}</p>
            </div>
          ),
        },
        {
          title: t('macOS 常见问题解决'),
          content: (
            <div className="space-y-4">
              <div>
                <p className="font-medium">{t('环境变量设置后不生效')}</p>
                <ul className="ml-4 space-y-1">
                  <li>• {t('重新打开终端窗口')}</li>
                  <li>• {t('或者执行：')}<InlineCode>source ~/.zshrc</InlineCode></li>
                </ul>
              </div>
            </div>
          ),
        },
      ],
      linux: [
        {
          title: t('安装 Node.js'),
          content: (
            <div className="space-y-4">
              <p>{t('Codex 需要 Node.js 18 或更高版本。')}</p>
              <p>{t('方法一：使用包管理器')}</p>
              <p className="font-medium">{t('Debian/Ubuntu：')}</p>
              <CodeBlock>{`sudo apt update\nsudo apt install nodejs npm`}</CodeBlock>
              <p className="font-medium">{t('CentOS/RHEL：')}</p>
              <CodeBlock>{`sudo dnf install nodejs npm`}</CodeBlock>
              <p>{t('方法二：使用 nvm（推荐）')}</p>
              <CodeBlock>{`curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash\nnvm install --lts`}</CodeBlock>
              <p>{t('验证安装：')}</p>
              <CodeBlock>{`node --version\nnpm --version`}</CodeBlock>
            </div>
          ),
        },
        {
          title: t('配置 Codex'),
          content: (
            <div className="space-y-4">
              <p>{t('编辑 ~/.bashrc：')}</p>
              <CodeBlock>{`echo 'export OPENAI_API_BASE="${serverAddress}"' >> ~/.bashrc\necho 'export OPENAI_API_KEY="sk_xxxxxxxxx"' >> ~/.bashrc\nsource ~/.bashrc`}</CodeBlock>
              <p className="text-yellow-400 text-sm">{t('记得将密钥替换为你的实际 API Key。')}</p>
              <p>{t('验证设置：')}</p>
              <CodeBlock>{`echo $OPENAI_API_BASE\necho $OPENAI_API_KEY`}</CodeBlock>
            </div>
          ),
        },
        {
          title: t('开始使用 Codex'),
          content: (
            <div className="space-y-4">
              <div className="bg-amber-900/30 rounded-lg">
                <p className="font-medium text-amber-400">{t('启动 Codex')}</p>
                <CodeBlock>{`codex`}</CodeBlock>
              </div>
              <div className="bg-gray-800/50 rounded-lg p-4">
                <p className="font-medium">{t('在项目中使用')}</p>
                <CodeBlock>{`# 进入你的项目目录\ncd /path/to/project\n\n# 启动 Codex\ncodex`}</CodeBlock>
              </div>
              <p className="text-green-400">{t('🎉 配置完成！')}</p>
            </div>
          ),
        },
        {
          title: t('Linux/WSL2 常见问题解决'),
          content: (
            <div className="space-y-4">
              <div>
                <p className="font-medium">{t('WSL2 中环境变量问题')}</p>
                <p className="ml-4">{t('确保在 WSL2 内部设置环境变量，而不是 Windows 系统环境变量')}</p>
              </div>
            </div>
          ),
        },
      ],
    },
    'gemini': {
      name: t('Gemini CLI'),
      windows: [
        {
          title: t('安装 Node.js'),
          content: (
            <div className="space-y-4">
              <p>{t('Gemini CLI 需要 Node.js 18 或更高版本。')}</p>
              <p>{t('方法一：官方安装包（推荐）')}</p>
              <CodeBlock>{`# 访问 https://nodejs.org 下载 LTS 版本安装包\n# 双击运行安装程序，按提示完成安装`}</CodeBlock>
              <p>{t('方法二：使用 Scoop 包管理器')}</p>
              <CodeBlock>{`scoop install nodejs-lts`}</CodeBlock>
              <p className="text-yellow-400 text-sm">{t('推荐使用官方安装包，安装过程简单且会自动配置环境变量。')}</p>
              <p>{t('验证安装：')}</p>
              <CodeBlock>{`node --version\nnpm --version`}</CodeBlock>
              <p>{t('如果显示版本号（如 v20.x.x 和 10.x.x），说明安装成功。')}</p>
            </div>
          ),
        },
        {
          title: t('配置 Gemini CLI'),
          content: (
            <div className="space-y-4">
              <p>{t('为了让 Gemini CLI 连接到中转服务，需要设置环境变量：')}</p>
              <p className="font-medium">{t('方法一：在系统环境变量中添加（推荐）')}</p>
              <p>{t('按 Win + R，输入 systempropertiesadvanced 回车，点击「环境变量」，在用户变量中新建：')}</p>
              <p>{t('变量名：')}<InlineCode>GOOGLE_API_KEY</InlineCode></p>
              <p>{t('变量值：')}<InlineCode>sk_xxxxxxxxx</InlineCode></p>
              <p className="text-yellow-400 text-sm">{t('添加后点击「确定」保存，然后重新打开终端窗口即可生效。')}</p>
              <p className="font-medium">{t('方法二：PowerShell 永久设置（用户级）')}</p>
              <CodeBlock>{`[System.Environment]::SetEnvironmentVariable("GOOGLE_API_KEY", "sk_xxxxxxxxx", [System.EnvironmentVariableTarget]::User)`}</CodeBlock>
              <p className="text-yellow-400 text-sm">{t('记得将密钥替换为你的实际 API Key。')}</p>
              <p>{t('验证环境变量设置：')}</p>
              <CodeBlock>{`echo $env:GOOGLE_API_KEY`}</CodeBlock>
            </div>
          ),
        },
        {
          title: t('开始使用 Gemini CLI'),
          content: (
            <div className="space-y-4">
              <div className="bg-blue-900/30 rounded-lg">
                <p className="font-medium text-blue-400">{t('启动 Gemini CLI')}</p>
                <CodeBlock>{`gemini`}</CodeBlock>
              </div>
              <div className="bg-gray-800/50 rounded-lg">
                <p className="font-medium">{t('在项目中使用')}</p>
                <CodeBlock>{`# 进入你的项目目录\ncd /path/to/project\n\n# 启动 Gemini CLI\ngemini`}</CodeBlock>
              </div>
              <p className="text-green-400">{t('🎉 配置完成！')}</p>
            </div>
          ),
        },
        {
          title: t('Windows 常见问题解决'),
          content: (
            <div className="space-y-4">
              <div>
                <p className="font-medium">{t('环境变量设置后不生效')}</p>
                <ul className="ml-4 space-y-1">
                  <li>• {t('重新启动 PowerShell 或 CMD')}</li>
                  <li>• {t('或者注销并重新登录 Windows')}</li>
                  <li>• {t('验证设置：')}<InlineCode>echo $env:GOOGLE_API_KEY</InlineCode></li>
                </ul>
              </div>
              <div>
                <p className="font-medium">{t('连接中转服务失败')}</p>
                <ul className="ml-4 space-y-1">
                  <li>• {t('确认 API Key 格式正确（以 cr_ 开头）')}</li>
                  <li>• {t('检查网络连接')}</li>
                </ul>
              </div>
            </div>
          ),
        },
      ],
      macos: [
        {
          title: t('安装 Node.js'),
          content: (
            <div className="space-y-4">
              <p>{t('Gemini CLI 需要 Node.js 18 或更高版本。')}</p>
              <p>{t('方法一：使用 Homebrew（推荐）')}</p>
              <CodeBlock>{`brew install node`}</CodeBlock>
              <p>{t('方法二：访问')} <a href="https://nodejs.org/" target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:underline">Node.js {t('官网')}</a> {t('下载安装')}</p>
              <p>{t('验证安装：')}</p>
              <CodeBlock>{`node --version\nnpm --version`}</CodeBlock>
            </div>
          ),
        },
        {
          title: t('配置 Gemini CLI'),
          content: (
            <div className="space-y-4">
              <p>{t('编辑 ~/.zshrc 或 ~/.bashrc：')}</p>
              <CodeBlock>{`echo 'export GOOGLE_API_KEY="sk_xxxxxxxxx"' >> ~/.zshrc\nsource ~/.zshrc`}</CodeBlock>
              <p className="text-yellow-400 text-sm">{t('记得将密钥替换为你的实际 API Key。')}</p>
              <p>{t('验证设置：')}</p>
              <CodeBlock>{`echo $GOOGLE_API_KEY`}</CodeBlock>
            </div>
          ),
        },
        {
          title: t('开始使用 Gemini CLI'),
          content: (
            <div className="space-y-4">
              <div className="bg-blue-900/30 rounded-lg">
                <p className="font-medium text-blue-400">{t('启动 Gemini CLI')}</p>
                <CodeBlock>{`gemini`}</CodeBlock>
              </div>
              <div className="bg-gray-800/50 rounded-lg">
                <p className="font-medium">{t('在项目中使用')}</p>
                <CodeBlock>{`# 进入你的项目目录\ncd /path/to/project\n\n# 启动 Gemini CLI\ngemini`}</CodeBlock>
              </div>
              <p className="text-green-400">{t('🎉 配置完成！')}</p>
            </div>
          ),
        },
        {
          title: t('macOS 常见问题解决'),
          content: (
            <div className="space-y-4">
              <div>
                <p className="font-medium">{t('环境变量设置后不生效')}</p>
                <ul className="ml-4 space-y-1">
                  <li>• {t('重新打开终端窗口')}</li>
                  <li>• {t('或者执行：')}<InlineCode>source ~/.zshrc</InlineCode></li>
                </ul>
              </div>
            </div>
          ),
        },
      ],
      linux: [
        {
          title: t('安装 Node.js'),
          content: (
            <div className="space-y-4">
              <p>{t('Gemini CLI 需要 Node.js 18 或更高版本。')}</p>
              <p>{t('方法一：使用包管理器')}</p>
              <p className="font-medium">{t('Debian/Ubuntu：')}</p>
              <CodeBlock>{`sudo apt update\nsudo apt install nodejs npm`}</CodeBlock>
              <p className="font-medium">{t('CentOS/RHEL：')}</p>
              <CodeBlock>{`sudo dnf install nodejs npm`}</CodeBlock>
              <p>{t('方法二：使用 nvm（推荐）')}</p>
              <CodeBlock>{`curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash\nnvm install --lts`}</CodeBlock>
              <p>{t('验证安装：')}</p>
              <CodeBlock>{`node --version\nnpm --version`}</CodeBlock>
            </div>
          ),
        },
        {
          title: t('配置 Gemini CLI'),
          content: (
            <div className="space-y-4">
              <p>{t('编辑 ~/.bashrc：')}</p>
              <CodeBlock>{`echo 'export GOOGLE_API_KEY="sk_xxxxxxxxx"' >> ~/.bashrc\nsource ~/.bashrc`}</CodeBlock>
              <p className="text-yellow-400 text-sm">{t('记得将密钥替换为你的实际 API Key。')}</p>
              <p>{t('验证设置：')}</p>
              <CodeBlock>{`echo $GOOGLE_API_KEY`}</CodeBlock>
            </div>
          ),
        },
        {
          title: t('开始使用 Gemini CLI'),
          content: (
            <div className="space-y-4">
              <div className="bg-blue-900/30 rounded-lg">
                <p className="font-medium text-blue-400">{t('启动 Gemini CLI')}</p>
                <CodeBlock>{`gemini`}</CodeBlock>
              </div>
              <div className="bg-gray-800/50 rounded-lg">
                <p className="font-medium">{t('在项目中使用')}</p>
                <CodeBlock>{`# 进入你的项目目录\ncd /path/to/project\n\n# 启动 Gemini CLI\ngemini`}</CodeBlock>
              </div>
              <p className="text-green-400">{t('🎉 配置完成！')}</p>
            </div>
          ),
        },
        {
          title: t('Linux/WSL2 常见问题解决'),
          content: (
            <div className="space-y-4">
              <div>
                <p className="font-medium">{t('WSL2 中环境变量问题')}</p>
                <p className="ml-4">{t('确保在 WSL2 内部设置环境变量，而不是 Windows 系统环境变量')}</p>
              </div>
            </div>
          ),
        },
      ],
    }
  };

  const currentSteps = tutorialData[activeTool]?.[activeOS] || [];

  // 切换工具或操作系统时，重置展开状态
  React.useEffect(() => {
    setOpenPanels([0]);
  }, [activeTool, activeOS]);

  // 根据主题获取背景色
  const getBackgroundColor = () => {
    return actualTheme === 'dark' ? '#11111b' : '#ffffff';
  };

  // 根据主题获取文本颜色
  const getTextColor = () => {
    return actualTheme === 'dark' ? '#e4e4e4' : '#1a1a1a';
  };

  // 根据主题获取次要文本颜色
  const getSecondaryTextColor = () => {
    return actualTheme === 'dark' ? '#999' : '#666';
  };

  return (
    <div 
      className="min-h-screen pb-16"
      style={{ 
        backgroundColor: getBackgroundColor(),
        paddingTop: isMobile ? '70px' : '80px',
      }}
    >
      <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8">
        {/* 标题区域 */}
        <div className="mb-8">
          <h1 
            className="text-2xl sm:text-3xl font-bold mb-2" 
            style={{ color: getTextColor() }}
          >
            {t('使用教程')}
          </h1>
          <p style={{ color: getSecondaryTextColor() }}>
            {t('选择工具，按步骤配置，开始使用 AI 编程助手')}
          </p>
        </div>

        {/* 工具选择标签 */}
        <div className="flex flex-wrap gap-2 mb-6">
          {tools.map((tool) => (
            <Button
              key={tool.key}
              theme={activeTool === tool.key ? 'solid' : 'borderless'}
              type={activeTool === tool.key ? 'primary' : 'tertiary'}
              onClick={() => setActiveTool(tool.key)}
              className="rounded-full px-6 py-2"
              style={{
                backgroundColor: activeTool === tool.key ? '#f59e0b' : (actualTheme === 'dark' ? '#2a2a3e' : '#f0f0f0'),
                borderColor: activeTool === tool.key ? '#f59e0b' : (actualTheme === 'dark' ? '#444' : '#ddd'),
                color: activeTool === tool.key ? '#111' : getTextColor(),
              }}
            >
              {tool.label}
            </Button>
          ))}
        </div>

        {/* 操作系统选择标签 */}
        <div className="flex flex-wrap gap-2 mb-8">
          {osOptions.map((os) => (
            <Button
              key={os.key}
              theme={activeOS === os.key ? 'solid' : 'borderless'}
              type={activeOS === os.key ? 'primary' : 'tertiary'}
              onClick={() => setActiveOS(os.key)}
              className="rounded-full px-4 py-2 text-sm"
              style={{
                backgroundColor: activeOS === os.key ? '#3b82f6' : (actualTheme === 'dark' ? '#2a2a3e' : '#f0f0f0'),
                borderColor: activeOS === os.key ? '#3b82f6' : (actualTheme === 'dark' ? '#444' : '#ddd'),
                color: getTextColor(),
              }}
            >
              {os.label}
            </Button>
          ))}
        </div>

        {/* 步骤卡片 */}
        <div className="space-y-2">
          {currentSteps.map((step, index) => (
            <CollapsiblePanel
              key={index}
              index={index}
              title={step.title}
              isOpen={openPanels.includes(index)}
              onToggle={() => handlePanelToggle(index)}
            >
              {step.content}
            </CollapsiblePanel>
          ))}
        </div>
      </div>
    </div>
  );
};

export default Tutorial;