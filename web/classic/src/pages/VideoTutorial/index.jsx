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
import { Button, Typography, Toast, Tag } from '@douyinfe/semi-ui';
import { IconChevronDown, IconCopy, IconInfoCircle, IconAlertTriangle } from '@douyinfe/semi-icons';
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
      className="px-1.5 py-0.5 rounded text-sm font-mono"
      style={{
        backgroundColor: isDark ? '#2a2a3e' : '#e8e8e8',
        color: isDark ? '#e4e4e4' : '#333',
      }}
    >
      {children}
    </code>
  );
};

// HTTP 方法徽章
const MethodBadge = ({ method }) => {
  const colors = {
    GET: '#3b82f6',
    POST: '#22c55e',
    DELETE: '#ef4444',
    PATCH: '#f59e0b',
    PUT: '#8b5cf6',
  };
  const bg = colors[method] || '#6b7280';
  return (
    <span
      className="inline-flex items-center justify-center flex-shrink-0"
      style={{
        backgroundColor: bg,
        color: '#fff',
        padding: '3px 9px',
        minWidth: '56px',
        borderRadius: '6px',
        fontSize: '11px',
        fontWeight: 700,
        letterSpacing: '0.6px',
        fontFamily: 'Consolas, Monaco, "Courier New", monospace',
      }}
    >
      {method}
    </span>
  );
};

// 接口路径头（方法 + 路径）
const EndpointHeader = ({ method, path }) => {
  const actualTheme = useActualTheme();
  const isDark = actualTheme === 'dark';
  return (
    <div
      className="flex items-center gap-2.5 px-3 py-2.5 my-3"
      style={{
        backgroundColor: isDark ? '#181825' : '#fafafa',
        border: `1px solid ${isDark ? '#2a2a3e' : '#e5e5e5'}`,
        borderRadius: '10px',
        overflow: 'auto',
      }}
    >
      <MethodBadge method={method} />
      <code
        className="text-sm font-mono"
        style={{ color: isDark ? '#e4e4e4' : '#1a1a1a', whiteSpace: 'nowrap' }}
      >
        {path}
      </code>
    </div>
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
        backgroundColor: '#0d1117',
        border: `1px solid ${isDark ? '#2a2a3e' : '#1f2430'}`,
        borderRadius: '10px',
        padding: '14px 16px',
        margin: '12px 0',
        overflow: 'auto',
        fontSize: '13.5px',
        lineHeight: '1.5',
        boxShadow: 'none',
      }}
      onMouseEnter={(e) => {
        const btn = e.currentTarget.querySelector('button');
        if (btn) btn.style.opacity = 1;
      }}
      onMouseLeave={(e) => {
        const btn = e.currentTarget.querySelector('button');
        if (btn) btn.style.opacity = 0.35;
      }}
    >
      <button
        onClick={handleCopy}
        style={{
          position: 'absolute',
          top: '10px',
          right: '10px',
          padding: '5px',
          backgroundColor: '#1c2333',
          borderRadius: '6px',
          border: `1px solid #2d3748`,
          cursor: 'pointer',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          opacity: 0.35,
          transition: 'opacity 0.2s ease',
        }}
      >
        <IconCopy size={14} style={{ color: '#8b98a9' }} />
      </button>
      <pre
        ref={ref}
        style={{
          margin: 0,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
          color: '#e6edf3',
          fontFamily: 'Consolas, Monaco, "Courier New", monospace',
          borderColor: 'inherit',
          boxShadow: 'none',
        }}
      >
        <code>{children}</code>
      </pre>
    </div>
  );
};

// 提示条组件（info / warn）
const Callout = ({ type = 'info', title, children }) => {
  const actualTheme = useActualTheme();
  const isDark = actualTheme === 'dark';

  const palette =
    type === 'warn'
      ? {
          accent: '#f59e0b',
          bg: isDark ? 'rgba(245,158,11,0.10)' : '#fffbeb',
          border: isDark ? 'rgba(245,158,11,0.35)' : '#fde68a',
          Icon: IconAlertTriangle,
        }
      : {
          accent: '#3b82f6',
          bg: isDark ? 'rgba(59,130,246,0.10)' : '#eff6ff',
          border: isDark ? 'rgba(59,130,246,0.35)' : '#bfdbfe',
          Icon: IconInfoCircle,
        };

  const { Icon } = palette;

  return (
    <div
      className="my-3 flex gap-3"
      style={{
        border: `1px solid ${palette.border}`,
        backgroundColor: palette.bg,
        borderRadius: '10px',
        padding: '12px 14px',
      }}
    >
      <Icon
        size="large"
        style={{ color: palette.accent, flexShrink: 0, marginTop: '1px' }}
      />
      <div className="min-w-0">
        {title && (
          <p
            className="font-semibold mb-1"
            style={{ color: isDark ? '#e4e4e4' : '#1a1a1a' }}
          >
            {title}
          </p>
        )}
        <div
          style={{ color: isDark ? '#c9c9c9' : '#57606a', fontSize: '14px' }}
        >
          {children}
        </div>
      </div>
    </div>
  );
};

// 小标题
const SectionLabel = ({ children }) => {
  const actualTheme = useActualTheme();
  const isDark = actualTheme === 'dark';
  return (
    <p
      className="font-semibold mt-5 mb-2"
      style={{
        color: isDark ? '#f0f0f0' : '#111',
        fontSize: '16px',
        letterSpacing: '-0.01em',
      }}
    >
      {children}
    </p>
  );
};

// 表格组件（firstColMono：首列等宽高亮，适合字段名）
const SimpleTable = ({ headers, rows, firstColMono = false }) => {
  const actualTheme = useActualTheme();
  const isDark = actualTheme === 'dark';
  const rowBorder = isDark ? '#2a2a3e' : '#ececf0';
  const outerBorder = isDark ? '#2a2a3e' : '#e5e5e5';
  const headBg = isDark ? '#181825' : '#fafafa';

  return (
    <div
      className="overflow-auto my-3"
      style={{
        border: `1px solid ${outerBorder}`,
        borderRadius: '10px',
      }}
    >
      <table
        style={{
          width: '100%',
          borderCollapse: 'collapse',
          fontSize: '13.5px',
        }}
      >
        <thead>
          <tr>
            {headers.map((h, i) => (
              <th
                key={i}
                style={{
                  padding: '10px 12px',
                  textAlign: 'left',
                  backgroundColor: headBg,
                  color: isDark ? '#e4e4e4' : '#1a1a1a',
                  fontWeight: 600,
                  whiteSpace: 'nowrap',
                  borderBottom: `1px solid ${outerBorder}`,
                }}
              >
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, ri) => (
            <tr key={ri}>
              {row.map((cell, ci) => {
                const mono = firstColMono && ci === 0;
                return (
                  <td
                    key={ci}
                    style={{
                      padding: '9px 12px',
                      color: mono
                        ? isDark
                          ? '#8be9fd'
                          : '#0550ae'
                        : isDark
                          ? '#b8b8c8'
                          : '#57606a',
                      verticalAlign: 'top',
                      fontFamily: mono
                        ? 'Consolas, Monaco, "Courier New", monospace'
                        : 'inherit',
                      whiteSpace: mono ? 'nowrap' : 'normal',
                      borderTop:
                        ri === 0 ? 'none' : `1px solid ${rowBorder}`,
                    }}
                  >
                    {cell}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};

// 自定义折叠面板组件
const CollapsiblePanel = ({ title, subtitle, children, isOpen, onToggle, index }) => {
  const actualTheme = useActualTheme();
  const isDark = actualTheme === 'dark';

  const getIconStyle = () => {
    const styles = [
      { bg: '#22c55e', color: '#fff' },
      { bg: '#f59e0b', color: '#fff' },
      { bg: '#3b82f6', color: '#fff' },
      { bg: '#8b5cf6', color: '#fff' },
      { bg: '#ef4444', color: '#fff' },
      { bg: '#06b6d4', color: '#fff' },
    ];
    return styles[index % styles.length];
  };

  const iconStyle = getIconStyle();

  return (
    <div
      className="border rounded-xl overflow-hidden mb-2.5"
      style={{
        backgroundColor: isDark ? '#1a1a2e' : '#ffffff',
        borderColor: isOpen
          ? isDark
            ? '#3d3d5c'
            : '#d0d7de'
          : isDark
            ? '#2a2a3e'
            : '#eaeaea',
      }}
    >
      <button
        onClick={onToggle}
        className="w-full px-4 py-3 flex items-center justify-between transition-colors"
        style={{
          textAlign: 'left',
          backgroundColor: 'transparent',
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.backgroundColor = isDark ? '#22223a' : '#f6f8fa';
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.backgroundColor = 'transparent';
        }}
      >
        <div className="flex items-center gap-3 min-w-0">
          <div
            className="w-7 h-7 rounded-lg flex items-center justify-center text-sm font-bold flex-shrink-0"
            style={{
              backgroundColor: iconStyle.bg,
              color: iconStyle.color,
            }}
          >
            {index + 1}
          </div>
          <div className="min-w-0">
            <Text
              className="font-medium block truncate"
              style={{ color: isDark ? '#e4e4e4' : '#1a1a1a' }}
            >
              {title}
            </Text>
            {subtitle && (
              <Text
                className="block truncate"
                size="small"
                style={{
                  color: isDark ? '#7a7a92' : '#8c959f',
                  fontSize: '12px',
                  fontFamily:
                    'Consolas, Monaco, "Courier New", monospace',
                }}
              >
                {subtitle}
              </Text>
            )}
          </div>
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
        <div
          className="px-4 pb-4 pt-2"
          style={{ color: isDark ? '#b0b0b0' : '#666' }}
        >
          {children}
        </div>
      )}
    </div>
  );
};

const VideoTutorial = () => {
  const { t } = useTranslation();
  const actualTheme = useActualTheme();
  const isMobile = useIsMobile();
  const [statusState] = useContext(StatusContext);
  const [activeProtocol, setActiveProtocol] = useState('volcengine');
  const [openPanels, setOpenPanels] = useState([0]);

  // 服务地址（Base URL）：优先使用本站配置的服务器地址，其次回退到当前站点地址
  const BASE_URL =
    statusState?.status?.server_address || `${window.location.origin}`;

  const protocols = [
    { key: 'volcengine', label: t('火山兼容 API') },
    { key: 'openai', label: t('OpenAI 兼容 API') },
    { key: 'assets', label: t('素材管理 API') },
  ];

  const handlePanelToggle = (index) => {
    setOpenPanels((prev) => {
      if (prev.includes(index)) {
        return prev.filter((i) => i !== index);
      }
      return [...prev, index];
    });
  };

  // ==================== 火山兼容 API ====================
  const volcengineSteps = [
    {
      title: t('快速接入'),
      subtitle: t('准备凭证、创建、轮询、取消删除'),
      content: (
        <div className="space-y-3">
          <SectionLabel>{t('1. 准备凭证')}</SectionLabel>
          <p>
            {t('服务地址固定为 ')}
            <InlineCode>{BASE_URL}</InlineCode>
            {t('，请求头使用平台分配的 API Key：')}
          </p>
          <CodeBlock>{`Authorization: Bearer <API_KEY>`}</CodeBlock>

          <SectionLabel>{t('2. 创建视频任务')}</SectionLabel>
          <CodeBlock>{`curl -X POST "${BASE_URL}/api/v3/contents/generations/tasks" \\
  -H "Authorization: Bearer <API_KEY>" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "content": [{ "type": "text", "text": "清晨的海边，一架纸飞机迎着风飞行" }],
    "duration": 5,
    "resolution": "720p",
    "ratio": "16:9"
  }'`}</CodeBlock>
          <p>{t('保存响应中的 id。')}</p>

          <SectionLabel>{t('3. 轮询结果')}</SectionLabel>
          <CodeBlock>{`curl "${BASE_URL}/api/v3/contents/generations/tasks/<TASK_ID>" \\
  -H "Authorization: Bearer <API_KEY>"`}</CodeBlock>
          <p>
            {t('任务状态为 queued 或 running 时继续轮询；进入 succeeded 或 failed 后停止。成功任务从 content.video_url 读取下载地址，并在 24 小时有效期内下载或转存。')}
          </p>

          <SectionLabel>{t('4. 取消或删除任务')}</SectionLabel>
          <p>
            {t('排队中或运行中的任务会被取消（本地标记为 failed）后删除；已结束的任务直接删除记录。删除任务不可恢复。')}
          </p>
          <CodeBlock>{`curl -X DELETE "${BASE_URL}/api/v3/contents/generations/tasks/<TASK_ID>" \\
  -H "Authorization: Bearer <API_KEY>"`}</CodeBlock>
          <Callout type="info" title={t('下一步')}>
            {t('图片、视频和音频参考输入请使用「创建视频生成任务」；需要复用媒体时，先在素材库接口中创建素材并取得可访问 URL。')}
          </Callout>
        </div>
      ),
    },
    {
      title: t('创建视频生成任务'),
      subtitle: 'POST /api/v3/contents/generations/tasks',
      content: (
        <div className="space-y-2">
          <EndpointHeader method="POST" path="/api/v3/contents/generations/tasks" />
          <p>
            {t('根据文本、图片、视频或音频创建异步视频生成任务。')}
          </p>
          <Callout type="warn" title={t('真人素材限制')}>
            {t('Seedance 2.0 不支持直接上传含真人人脸的参考图片或视频。请使用平台支持的含人脸原始产物、预置虚拟人像，或通过素材库完成授权的真人素材。')}
          </Callout>

          <SectionLabel>{t('请求参数')}</SectionLabel>
          <SimpleTable
            firstColMono
            headers={[t('字段'), t('类型'), t('必选'), t('说明')]}
            rows={[
              ['model', 'string', t('是'), t('模型 id 或者模型名称。')],
              ['content', 'object[]', t('是'), t('视频生成输入，支持文本、图片、视频和音频。')],
              ['content.type', 'string', t('是'), 'text、image_url、video_url 或 audio_url。'],
              ['content.text', 'string', t('条件必选'), t('文本提示词，建议中文不超过 500 字、英文不超过 1000 词。')],
              ['content.image_url.url', 'string', t('条件必选'), t('公网图片 URL、Base64 Data URL 或 asset://<ASSET_ID>。')],
              ['content.video_url.url', 'string', t('条件必选'), t('公网视频 URL 或 asset://<ASSET_ID>。')],
              ['content.audio_url.url', 'string', t('条件必选'), t('参考音频 URL；仅 Seedance 2.0，且不能单独输入音频。')],
              ['content.role', 'string', t('条件必选'), 'first_frame、last_frame、reference_image、reference_video 或 reference_audio。'],
              ['callback_url', 'string', t('否'), t('任务状态变化时接收 POST 回调；回调体与查询任务响应一致。')],
              ['return_last_frame', 'boolean', t('否'), t('默认 false；是否返回无水印尾帧图片。')],
              ['service_tier', 'string', t('否'), t('默认 default；default 为在线推理，flex 为离线推理。Seedance 2.0 仅支持在线推理。')],
              ['execution_expires_after', 'integer', t('否'), t('任务过期秒数，默认 172800，范围为 3600–259200。')],
              ['generate_audio', 'boolean', t('否'), t('默认 true；Seedance 2.0/1.5 Pro 是否生成同步单声道音频。')],
              ['draft', 'boolean', t('否'), t('默认 false；仅 Seedance 1.5 Pro，开启后生成 480p 样片。')],
              ['tools', 'object[]', t('否'), t('仅 Seedance 2.0；当前可配置 web_search。')],
              ['safety_identifier', 'string', t('否'), t('固定且唯一的终端用户标识，最长 64 个英文字符，建议传入哈希值。')],
              ['priority', 'integer', t('否'), t('仅 Seedance 2.0，范围 0–9；数值越大，同一 Endpoint 内排队越靠前。')],
              ['resolution', 'string', t('否'), t('480p、720p、1080p；Seedance 2.0 还支持 4k。')],
              ['ratio', 'string', t('否'), '16:9、4:3、1:1、3:4、9:16、21:9 或 adaptive。'],
              ['duration', 'integer', t('否'), t('视频时长（秒），默认 5；与 frames 二选一。')],
              ['frames', 'integer', t('否'), t('生成帧数，优先级高于 duration；Seedance 2.0/1.5 Pro 暂不支持。')],
              ['seed', 'integer', t('否'), t('默认 -1，范围 [-1, 2^32-1]；Seedance 2.0 暂不支持。')],
              ['camera_fixed', 'boolean', t('否'), t('默认 false；参考图场景和 Seedance 2.0 暂不支持。')],
              ['watermark', 'boolean', t('否'), t('默认 false；设为 true 时在右下角展示「AI 生成」水印。')],
            ]}
          />

          <SectionLabel>{t('输入组合')}</SectionLabel>
          <ul className="ml-4 space-y-1">
            <li>• {t('纯文本。')}</li>
            <li>• {t('文本（可选）+ 图片。')}</li>
            <li>• {t('文本（可选）+ 视频。')}</li>
            <li>• {t('文本（可选）+ 图片或视频 + 音频。')}</li>
          </ul>
          <p className="text-sm">
            {t('首帧、首尾帧和多模态参考是三种互斥场景，不可混用。图片支持公网 URL、Base64 Data URL 和素材 ID；视频支持公网 URL 和素材 ID。')}
          </p>

          <SectionLabel>{t('素材限制')}</SectionLabel>
          <ul className="ml-4 space-y-1 text-sm">
            <li>• {t('单张图片小于 30 MB，请求体不超过 64 MB；宽高比范围为 [0.4, 2.5]，边长范围为 300–6000 px。')}</li>
            <li>• {t('单个视频不超过 200 MB，时长为 2–15 秒；最多传入 3 个参考视频，且总时长不超过 15 秒。')}</li>
            <li>• {t('execution_expires_after 到期后，仍在排队或运行中的任务会变为 expired。')}</li>
            <li>• {t('回调状态包括 queued、running、succeeded、failed 和 expired；成功或失败回调发送失败时最多重试 3 次。')}</li>
          </ul>

          <SectionLabel>{t('请求示例')}</SectionLabel>
          <CodeBlock>{`curl -X POST "${BASE_URL}/api/v3/contents/generations/tasks" \\
  -H "Authorization: Bearer <API_KEY>" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "content": [
      {"type": "text", "text": "一艘木船穿过晨雾中的湖面，写实电影镜头"}
    ],
    "resolution": "720p",
    "ratio": "16:9",
    "duration": 5,
    "generate_audio": true
  }'`}</CodeBlock>

          <SectionLabel>{t('响应内容')}</SectionLabel>
          <p className="text-sm">
            {t('任务创建成功后返回异步任务标识。任务记录保存 7 天，请保存 id，用于查询、列举或取消任务。')}
          </p>
          <SimpleTable
            firstColMono
            headers={[t('字段'), t('类型'), t('说明')]}
            rows={[
              ['id', 'string', t('视频生成任务 ID。')],
              ['safety_identifier', 'string', t('请求中传入的终端用户标识；未传入时不返回。')],
            ]}
          />
          <CodeBlock>{`{
  "id": "cgt-2026-example"
}`}</CodeBlock>
        </div>
      ),
    },
    {
      title: t('查询视频生成任务'),
      subtitle: 'GET /api/v3/contents/generations/tasks/{task_id}',
      content: (
        <div className="space-y-2">
          <EndpointHeader method="GET" path="/api/v3/contents/generations/tasks/{task_id}" />
          <p>{t('根据任务 ID 查询视频生成任务的状态和输出内容。')}</p>
          <Callout type="info" title={t('查询范围与视频地址有效期')}>
            {t('仅支持查询最近 7 天的任务记录，时间区间为 [T-7 天, T)，其中 T 为请求发起时刻的 UTC 时间戳（精确到秒）。响应中的视频和尾帧 URL 有效期为 24 小时，请及时下载或转存。')}
          </Callout>

          <SectionLabel>{t('请求参数')}</SectionLabel>
          <SimpleTable
            firstColMono
            headers={['Path', t('类型'), t('必选'), t('说明')]}
            rows={[['task_id', 'string', t('是'), t('创建任务接口返回的视频生成任务 ID。')]]}
          />
          <CodeBlock>{`curl "${BASE_URL}/api/v3/contents/generations/tasks/cgt-2026-example" \\
  -H "Authorization: Bearer <API_KEY>"`}</CodeBlock>

          <SectionLabel>{t('响应内容')}</SectionLabel>
          <SimpleTable
            firstColMono
            headers={[t('字段'), t('类型'), t('说明')]}
            rows={[
              ['id', 'string', t('视频生成任务 ID。')],
              ['model', 'string', t('任务使用的模型名称和版本。')],
              ['status', 'string', 'queued、running、succeeded 或 failed。'],
              ['error', 'object', t('失败时包含 code 和 message。')],
              ['created_at', 'integer', t('任务创建时间的 Unix 时间戳（秒）。')],
              ['updated_at', 'integer', t('任务状态更新时间的 Unix 时间戳（秒）。')],
              ['content.video_url', 'string', t('生成视频 MP4 URL，有效期为 24 小时。')],
              ['seed', 'integer', t('本次请求使用的种子。')],
              ['resolution', 'string', t('实际生成视频的分辨率。')],
              ['ratio', 'string', t('实际生成视频的宽高比。')],
              ['duration', 'integer', t('视频时长（秒）。')],
              ['framespersecond', 'integer', t('生成视频帧率。')],
              ['service_tier', 'string', t('实际处理任务使用的服务等级。')],
              ['tools', 'object[]', t('模型实际使用的工具；未使用工具时不返回。')],
              ['usage.completion_tokens', 'integer', t('生成视频消耗的 token 数量，可用于计费对账。')],
              ['usage.total_tokens', 'integer', t('总 token 数；视频任务不统计输入 token。')],
              ['usage.tool_usage.web_search', 'integer', t('联网搜索工具的实际调用次数。')],
            ]}
          />
          <CodeBlock>{`{
  "id": "cgt-2026-example",
  "model": "doubao-seedance-2-0-260128",
  "status": "succeeded",
  "content": {
    "video_url": "https://example.com/video.mp4"
  },
  "duration": 5,
  "resolution": "720p",
  "ratio": "16:9",
  "created_at": 1784718000,
  "updated_at": 1784718060,
  "usage": {
    "completion_tokens": 1000,
    "total_tokens": 1000
  }
}`}</CodeBlock>
        </div>
      ),
    },
    {
      title: t('查询视频生成任务列表'),
      subtitle: 'GET /api/v3/contents/generations/tasks',
      content: (
        <div className="space-y-2">
          <EndpointHeader method="GET" path="/api/v3/contents/generations/tasks" />
          <p>{t('通过分页和筛选参数查询视频生成任务。')}</p>
          <Callout type="info" title={t('查询范围与视频地址有效期')}>
            {t('仅支持查询最近 7 天的任务记录，时间区间为 [T-7 天, T)，其中 T 为请求发起时刻的 UTC 时间戳（精确到秒）。响应中的视频和尾帧 URL 有效期为 24 小时，请及时下载或转存。')}
          </Callout>

          <SectionLabel>{t('请求参数')}</SectionLabel>
          <p className="text-sm">
            {t('本接口为 GET 请求，没有请求体。所有筛选条件均通过 Query 参数传递。')}
          </p>
          <SimpleTable
            firstColMono
            headers={['Query', t('类型'), t('默认值'), t('说明')]}
            rows={[
              ['page_size', 'integer', '20', t('每页返回的任务数量。')],
              ['page_token', 'integer', '-', t('分页游标，传入上一页返回的 next_page_token 获取下一页。')],
              ['created_after', 'integer', '-', t('仅返回该 Unix 时间戳（秒）之后创建的任务。')],
              ['status', 'string', '-', t('任务状态过滤，多个状态用英文逗号分隔。')],
              ['platform', 'string', '-', t('可选，按平台过滤；火山原生入口默认不带。')],
            ]}
          />
          <CodeBlock>{`curl -X GET "${BASE_URL}/api/v3/contents/generations/tasks?page_size=3&status=succeeded" \\
  -H "Authorization: Bearer <API_KEY>"`}</CodeBlock>

          <SectionLabel>{t('响应内容')}</SectionLabel>
          <p className="text-sm">{t('响应根对象包含：')}</p>
          <SimpleTable
            firstColMono
            headers={[t('字段'), t('类型'), t('说明')]}
            rows={[
              ['items', 'object[]', t('查询到的视频生成任务列表。')],
              ['next_page_token', 'string', t('下一页游标；为空字符串表示没有更多数据。')],
            ]}
          />
          <p className="text-sm">{t('每个 items 元素可能包含：')}</p>
          <SimpleTable
            firstColMono
            headers={[t('字段'), t('类型'), t('说明')]}
            rows={[
              ['id', 'string', t('视频生成任务 ID。')],
              ['model', 'string', t('任务使用的模型名称和版本。')],
              ['status', 'string', 'queued、running、succeeded 或 failed。'],
              ['error', 'object', t('失败时包含 code 和 message。')],
              ['created_at', 'integer', t('任务创建时间的 Unix 时间戳（秒）。')],
              ['updated_at', 'integer', t('任务状态更新时间的 Unix 时间戳（秒）。')],
              ['content.video_url', 'string', t('生成视频 URL，有效期为 24 小时。')],
              ['seed', 'integer', t('本次请求使用的种子整数值。')],
              ['resolution', 'string', t('生成视频的分辨率。')],
              ['ratio', 'string', t('生成视频的宽高比。')],
              ['duration', 'integer', t('视频时长（秒）。')],
              ['framespersecond', 'integer', t('视频帧率。')],
              ['tools', 'object[]', t('模型实际使用的工具；当前工具类型包括 web_search。')],
              ['service_tier', 'string', t('实际处理任务使用的服务等级。')],
              ['usage.completion_tokens', 'integer', t('模型生成视频消耗的 token 数量，可用于计费对账。')],
              ['usage.total_tokens', 'integer', t('总 token 数量；视频任务中等于 completion_tokens。')],
              ['usage.tool_usage.web_search', 'integer', t('实际调用联网搜索工具的次数。')],
            ]}
          />
        </div>
      ),
    },
    {
      title: t('取消或删除视频生成任务'),
      subtitle: 'DELETE /api/v3/contents/generations/tasks/{task_id}',
      content: (
        <div className="space-y-2">
          <EndpointHeader method="DELETE" path="/api/v3/contents/generations/tasks/{task_id}" />
          <p>{t('根据任务当前状态取消排队任务，或删除已结束的任务记录。')}</p>
          <Callout type="warn" title={t('删除不可恢复')}>
            {t('删除任务记录后无法再次查询。需要保留生成视频时，请先下载或转存有效期为 24 小时的视频 URL。')}
          </Callout>

          <SectionLabel>{t('请求参数')}</SectionLabel>
          <SimpleTable
            firstColMono
            headers={['Path', t('类型'), t('必选'), t('说明')]}
            rows={[['task_id', 'string', t('是'), t('需要取消或删除的视频生成任务 ID。')]]}
          />
          <CodeBlock>{`curl -X DELETE "${BASE_URL}/api/v3/contents/generations/tasks/cgt-2026-example" \\
  -H "Authorization: Bearer <API_KEY>"`}</CodeBlock>

          <SectionLabel>{t('状态与操作')}</SectionLabel>
          <SimpleTable
            firstColMono
            headers={[t('当前状态'), t('处理结果')]}
            rows={[
              ['queued', t('取消排队（本地标记为 failed）后删除任务记录。')],
              ['running', t('中断任务（本地标记为 failed）后删除任务记录。')],
              ['succeeded', t('删除任务记录，之后无法查询。')],
              ['failed', t('删除任务记录，之后无法查询。')],
            ]}
          />

          <SectionLabel>{t('响应内容')}</SectionLabel>
          <p className="text-sm">{t('操作成功时返回被删除的任务 ID：')}</p>
          <CodeBlock>{`{
  "id": "cgt-2026-example",
  "deleted": true
}`}</CodeBlock>
        </div>
      ),
    },
  ];

  // ==================== OpenAI 兼容 API ====================
  const openaiSteps = [
    {
      title: t('模型与请求参数'),
      subtitle: t('Seedance 字段白名单'),
      content: (
        <div className="space-y-2">
          <p>{t('创建接口使用严格的 Seedance 字段白名单，并且必须传入 model。')}</p>

          <SectionLabel>{t('请求字段')}</SectionLabel>
          <SimpleTable
            firstColMono
            headers={[t('字段'), t('类型'), t('创建必填'), t('说明')]}
            rows={[
              ['model', 'string', t('是'), 'doubao-seedance-2-0-260128 或 doubao-seedance-2-0-fast-260128'],
              ['prompt', 'string', t('是'), t('非空视频提示词')],
              ['duration', 'integer/string', t('否'), t('时长，1-3600 秒')],
              ['seconds', 'string', t('否'), t('OpenAI 兼容时长，内容必须是整数，1-3600')],
              ['resolution', 'string', t('否'), t('480p、720p、1080p；Fast 模型不支持 1080p')],
              ['ratio', 'string', t('否'), '16:9、9:16、1:1'],
              ['image', 'string', t('否'), t('单张参考图 URL')],
              ['images', 'string[]', t('否'), t('多张参考图 URL')],
              ['generate_audio', 'boolean', t('否'), t('是否生成音频；显式 false 会保留')],
              ['watermark', 'boolean', t('否'), t('是否添加水印')],
              ['return_last_frame', 'boolean', t('否'), t('是否返回尾帧')],
              ['camera_fixed', 'boolean', t('否'), t('是否固定镜头')],
              ['safety_identifier', 'string', t('否'), t('调用方安全标识，最多 64 个字符')],
              ['metadata', 'object', t('否'), t('多模态内容和高级参数')],
            ]}
          />
          <p className="text-sm">
            {t('不支持的顶层字段返回 unsupported_field。顶层 content、priority 和 input_reference 不属于当前契约。')}
          </p>

          <SectionLabel>{t('参数优先级')}</SectionLabel>
          <p className="text-sm">{t('重复字段按以下顺序解析：')}</p>
          <CodeBlock>{`duration > seconds > metadata.duration
顶层 resolution > metadata.resolution
顶层 ratio > metadata.ratio
顶层布尔字段 > metadata 中的同名布尔字段
metadata.content > 顶层 image / images`}</CodeBlock>
          <p className="text-sm">{t('metadata 只允许以下字段：')}</p>
          <CodeBlock>{`content
duration
resolution
ratio
generate_audio
watermark
return_last_frame
camera_fixed`}</CodeBlock>
          <p className="text-sm">{t('出现其他 metadata 字段会返回 invalid_metadata。')}</p>

          <SectionLabel>{t('顶层图片参考')}</SectionLabel>
          <CodeBlock>{`{
  "model": "doubao-seedance-2-0-260128",
  "prompt": "保持产品主体一致，生成电影感广告运镜",
  "duration": 8,
  "resolution": "720p",
  "ratio": "16:9",
  "images": [
    "https://cdn.example.com/product-front.png",
    "https://cdn.example.com/product-side.png"
  ]
}`}</CodeBlock>

          <SectionLabel>metadata.content</SectionLabel>
          <SimpleTable
            firstColMono
            headers={['type', 'URL', 'role']}
            rows={[
              ['image_url', 'image_url.url', t('空、reference_image、first_frame、last_frame')],
              ['video_url', 'video_url.url', t('空、reference_image、first_frame、last_frame')],
              ['audio_url', 'audio_url.url', t('空、reference_image、first_frame、last_frame')],
              ['text', 'text', t('不支持作为提示词；该项会被忽略')],
            ]}
          />
          <p className="text-sm">
            {t('每个 URL 对象只允许包含 url 字段，不支持自定义请求头。最终文本提示词始终使用顶层 prompt。')}
          </p>
          <CodeBlock>{`{
  "model": "doubao-seedance-2-0-260128",
  "prompt": "保持人物动作，替换为傍晚海边场景",
  "duration": 8,
  "resolution": "720p",
  "ratio": "16:9",
  "metadata": {
    "content": [
      {
        "type": "video_url",
        "video_url": { "url": "https://cdn.example.com/reference.mp4" },
        "role": "reference_image"
      },
      {
        "type": "audio_url",
        "audio_url": { "url": "https://cdn.example.com/reference.mp3" }
      }
    ]
  }
}`}</CodeBlock>
          <p className="text-sm">
            {t('只要 metadata.content 中存在 video_url，任务就会按参考视频场景记录和计费。媒体必须是服务端可访问的公网 URL，不能直接通过 multipart 上传。')}
          </p>
        </div>
      ),
    },
    {
      title: t('创建 Seedance 视频'),
      subtitle: 'POST /v1/videos',
      content: (
        <div className="space-y-2">
          <EndpointHeader method="POST" path="/v1/videos" />
          <p>
            {t('仅接受 application/json，不接受 multipart 文件字段。model 和非空 prompt 必填。创建请求通常返回 200；网关已接收但仍需异步确认时可能返回 202。')}
          </p>
          <p className="text-sm">
            {t('完整请求字段见「模型与请求参数」。创建成功后保存响应中的 id，id 与兼容字段 task_id 相同，不会暴露上游真实任务 ID。')}
          </p>

          <SectionLabel>{t('请求示例')}</SectionLabel>
          <CodeBlock>{`curl -X POST "${BASE_URL}/v1/videos" \\
  -H "Authorization: Bearer <API_KEY>" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "prompt": "雨后的未来城市，镜头缓慢向前推进",
    "seconds": "8",
    "resolution": "720p",
    "ratio": "16:9"
  }'`}</CodeBlock>

          <SectionLabel>{t('响应内容')}</SectionLabel>
          <p className="text-sm">{t('返回 OpenAI 风格的视频对象。')}</p>
          <CodeBlock>{`{
  "id": "task_public_id",
  "object": "video",
  "model": "doubao-seedance-2-0-260128",
  "status": "queued",
  "progress": 0,
  "created_at": 1784600000
}`}</CodeBlock>
        </div>
      ),
    },
    {
      title: t('查询 Seedance 视频'),
      subtitle: 'GET /v1/videos/{task_id}',
      content: (
        <div className="space-y-2">
          <EndpointHeader method="GET" path="/v1/videos/{task_id}" />
          <p>{t('根据任务 ID 查询视频对象的状态和结果。')}</p>

          <SectionLabel>{t('请求参数')}</SectionLabel>
          <SimpleTable
            firstColMono
            headers={['Path', t('类型'), t('必选'), t('说明')]}
            rows={[['task_id', 'string', t('是'), t('创建接口返回的本平台 task_... ID。')]]}
          />
          <CodeBlock>{`curl "${BASE_URL}/v1/videos/<TASK_ID>" \\
  -H "Authorization: Bearer <API_KEY>"`}</CodeBlock>

          <SectionLabel>{t('任务状态')}</SectionLabel>
          <SimpleTable
            firstColMono
            headers={['status', t('说明')]}
            rows={[
              ['queued', t('已排队')],
              ['in_progress', t('生成中')],
              ['completed', t('已完成')],
              ['failed', t('失败')],
              ['unknown', t('未知状态')],
            ]}
          />
          <p className="text-sm">
            {t('completed 和 failed 是终态。建议从 2 秒轮询间隔开始，并逐步增加到 5-10 秒。成功后读取 result_url 或 metadata.url，也可以调用内容接口下载视频。')}
          </p>

          <SectionLabel>{t('响应示例')}</SectionLabel>
          <CodeBlock>{`{
  "id": "task_public_id",
  "object": "video",
  "model": "doubao-seedance-2-0-260128",
  "status": "completed",
  "progress": 100,
  "created_at": 1784600000,
  "completed_at": 1784600060,
  "result_url": "https://example.com/video.mp4"
}`}</CodeBlock>
        </div>
      ),
    },
    {
      title: t('下载视频内容'),
      subtitle: 'GET /v1/videos/{task_id}/content',
      content: (
        <div className="space-y-2">
          <EndpointHeader method="GET" path="/v1/videos/{task_id}/content" />
          <p>{t('下载或代理读取已完成视频的二进制内容。')}</p>

          <SectionLabel>{t('请求参数')}</SectionLabel>
          <SimpleTable
            firstColMono
            headers={['Path', t('类型'), t('必选'), t('说明')]}
            rows={[['task_id', 'string', t('是'), t('已完成任务的本平台 task_... ID。')]]}
          />
          <CodeBlock>{`curl "${BASE_URL}/v1/videos/<TASK_ID>/content" \\
  -H "Authorization: Bearer <API_KEY>" \\
  -o video.mp4`}</CodeBlock>
          <Callout type="info">
            {t('仅已完成任务可下载内容；未完成任务请先轮询查询接口确认 status 为 completed。')}
          </Callout>
        </div>
      ),
    },
    {
      title: t('错误码'),
      subtitle: t('顶层 error 对象'),
      content: (
        <div className="space-y-2">
          <p>{t('OpenAI 兼容接口使用顶层 error 对象返回请求错误：')}</p>
          <CodeBlock>{`{
  "error": {
    "message": "Task not found",
    "type": "invalid_request_error",
    "param": null,
    "code": "task_not_exist"
  }
}`}</CodeBlock>

          <SectionLabel>{t('Seedance 请求错误')}</SectionLabel>
          <SimpleTable
            firstColMono
            headers={['code', 'HTTP', t('说明')]}
            rows={[
              ['invalid_request', '400', t('JSON、prompt 或基础字段无效')],
              ['unsupported_field', '400', t('出现不支持的顶层字段或 multipart 文件')],
              ['invalid_metadata', '400', t('metadata 类型或字段无效')],
              ['invalid_content', '400', t('metadata.content 项、URL 对象或 role 无效')],
              ['invalid_seconds', '400', t('时长不是合法正整数或超过 3600 秒')],
              ['invalid_resolution', '400', t('分辨率不支持，或 Fast 模型使用了 1080p')],
              ['invalid_ratio', '400', t('比例不是 16:9、9:16 或 1:1')],
              ['invalid_safety_identifier', '400', t('安全标识超过 64 个字符')],
              ['task_not_exist', '400/404', t('任务不存在或不属于当前用户')],
              ['seedance_multi_key_unsupported', '400', t('Seedance 渠道启用了无法稳定关联凭证的多 Key 模式')],
            ]}
          />

          <SectionLabel>{t('任务失败')}</SectionLabel>
          <p className="text-sm">
            {t('任务创建成功后仍可能异步失败。此时查询接口返回 HTTP 200，但视频对象的 status 为 failed，失败原因位于 error.code 和 error.message：')}
          </p>
          <CodeBlock>{`{
  "id": "task_public_id",
  "object": "video",
  "model": "doubao-seedance-2-0-260128",
  "status": "failed",
  "progress": 100,
  "created_at": 1784600000,
  "completed_at": 1784600030,
  "error": {
    "code": "task_failed",
    "message": "Task failed"
  }
}`}</CodeBlock>
          <Callout type="warn">
            {t('客户端必须同时检查 HTTP 状态和任务对象的 status。')}
          </Callout>
        </div>
      ),
    },
  ];

  // ==================== 素材管理 API ====================
  const assetSteps = [
    {
      title: t('快速接入'),
      subtitle: t('鉴权、建组、上传素材、轮询状态'),
      content: (
        <div className="space-y-3">
          <SectionLabel>{t('1. 准备凭证')}</SectionLabel>
          <p>
            {t('素材管理接口属于控制台接口，使用「个人设置」中生成的系统访问令牌鉴权。需同时携带两个请求头：Authorization 为访问令牌（无 Bearer 前缀），New-Api-User 为你的用户 ID：')}
          </p>
          <CodeBlock>{`Authorization: <ACCESS_TOKEN>
New-Api-User: <USER_ID>`}</CodeBlock>
          <p className="text-sm">
            {t('接口前缀统一为 /api/asset-library，请求与响应均为 application/json，字段使用小写下划线命名。')}
          </p>

          <SectionLabel>{t('2. 创建素材组')}</SectionLabel>
          <CodeBlock>{`curl -X POST "${BASE_URL}/api/asset-library/groups" \\
  -H "Authorization: <ACCESS_TOKEN>" \\
  -H "New-Api-User: <USER_ID>" \\
  -H "Content-Type: application/json" \\
  -d '{ "display_name": "my-group", "group_type": "AIGC", "description": "" }'`}</CodeBlock>
          <p>{t('从响应 data.group.id 取得素材组 ID。')}</p>

          <SectionLabel>{t('3. 上传素材（公网 URL）')}</SectionLabel>
          <CodeBlock>{`curl -X POST "${BASE_URL}/api/asset-library/groups/<GROUP_ID>/assets" \\
  -H "Authorization: <ACCESS_TOKEN>" \\
  -H "New-Api-User: <USER_ID>" \\
  -H "Content-Type: application/json" \\
  -d '{ "assets": [ { "url": "https://example.com/logo.png" } ] }'`}</CodeBlock>
          <p className="text-sm">
            {t('素材为异步入库，创建后需轮询状态。')}
          </p>

          <SectionLabel>{t('4. 轮询素材状态')}</SectionLabel>
          <CodeBlock>{`curl -X POST "${BASE_URL}/api/asset-library/groups/<GROUP_ID>/refresh" \\
  -H "Authorization: <ACCESS_TOKEN>" \\
  -H "New-Api-User: <USER_ID>"`}</CodeBlock>
          <p>
            {t('刷新会拉取最新状态并回写；随后用「查询素材组详情」查看每个素材的 status 与 asset_url。')}
          </p>

          <Callout type="info" title={t('复用到视频生成')}>
            {t('素材状态变为 Active 后，可在「创建视频生成任务」的 content 中用 asset://<ASSET_ID> 引用；此处的 ASSET_ID 为素材详情中返回的 asset_id。')}
          </Callout>
        </div>
      ),
    },
    {
      title: t('查询素材组列表'),
      subtitle: 'GET /api/asset-library/groups',
      content: (
        <div className="space-y-2">
          <EndpointHeader method="GET" path="/api/asset-library/groups" />
          <p>{t('返回当前用户的全部素材组及组内素材。')}</p>
          <SectionLabel>{t('响应内容')}</SectionLabel>
          <SimpleTable
            firstColMono
            headers={[t('字段'), t('类型'), t('说明')]}
            rows={[
              ['data', 'object[]', t('素材组数组。')],
              ['data[].id', 'integer', t('素材组 ID。')],
              ['data[].display_name', 'string', t('素材组名称。')],
              ['data[].description', 'string', t('素材组描述。')],
              ['data[].group_type', 'string', t('素材组类型，当前固定为 AIGC。')],
              ['data[].status', 'string', t('素材组聚合状态：Active、Processing 或 Failed。')],
              ['data[].assets', 'object[]', t('组内素材列表。')],
              ['data[].assets[].id', 'integer', t('素材 ID。')],
              ['data[].assets[].name', 'string', t('素材名称。')],
              ['data[].assets[].status', 'string', t('素材聚合状态：Active、Processing 或 Failed。')],
              ['data[].assets[].asset_url', 'string', t('素材访问地址（有效期 12 小时）。')],
              ['data[].assets[].asset_id', 'string', t('素材资产 ID，可用于 asset:// 引用。')],
            ]}
          />
          <p className="text-sm">
            {t('查询单个素材组详情：GET /api/asset-library/groups/{id}。')}
          </p>
        </div>
      ),
    },
    {
      title: t('创建素材组'),
      subtitle: 'POST /api/asset-library/groups',
      content: (
        <div className="space-y-2">
          <EndpointHeader method="POST" path="/api/asset-library/groups" />
          <p>{t('创建一个空素材组，随后再向组内追加素材。')}</p>
          <SectionLabel>{t('请求参数')}</SectionLabel>
          <SimpleTable
            firstColMono
            headers={[t('字段'), t('类型'), t('必选'), t('说明')]}
            rows={[
              ['display_name', 'string', t('是'), t('素材组名称，最长 64 个字符。')],
              ['group_type', 'string', t('否'), t('默认 AIGC，当前仅支持 AIGC。')],
              ['description', 'string', t('否'), t('素材组描述，最长 300 个字符。')],
            ]}
          />
          <SectionLabel>{t('请求示例')}</SectionLabel>
          <CodeBlock>{`curl -X POST "${BASE_URL}/api/asset-library/groups" \\
  -H "Authorization: <ACCESS_TOKEN>" \\
  -H "New-Api-User: <USER_ID>" \\
  -H "Content-Type: application/json" \\
  -d '{ "display_name": "product-shots", "group_type": "AIGC" }'`}</CodeBlock>
          <SectionLabel>{t('响应内容')}</SectionLabel>
          <p className="text-sm">
            {t('data.group 为新建素材组，data.results 为各上游建组结果（success 与 message）。')}
          </p>
        </div>
      ),
    },
    {
      title: t('上传素材'),
      subtitle: 'POST /api/asset-library/groups/{id}/assets',
      content: (
        <div className="space-y-2">
          <EndpointHeader method="POST" path="/api/asset-library/groups/{id}/assets" />
          <p>
            {t('向素材组追加素材，通过 JSON 传入公网可访问的素材 URL。素材异步入库。')}
          </p>
          <SimpleTable
            firstColMono
            headers={[t('字段'), t('类型'), t('必选'), t('说明')]}
            rows={[
              ['assets', 'object[]', t('是'), t('素材数组，最多 20 个。')],
              ['assets[].url', 'string', t('是'), t('公网可访问的素材 URL。')],
              ['assets[].name', 'string', t('否'), t('素材名称；缺省时从 URL 推断。')],
              ['assets[].asset_type', 'string', t('否'), t('Image、Video 或 Audio；缺省时按扩展名推断。')],
            ]}
          />
          <CodeBlock>{`curl -X POST "${BASE_URL}/api/asset-library/groups/<GROUP_ID>/assets" \\
  -H "Authorization: <ACCESS_TOKEN>" \\
  -H "New-Api-User: <USER_ID>" \\
  -H "Content-Type: application/json" \\
  -d '{ "assets": [ { "url": "https://example.com/a.png", "asset_type": "Image" } ] }'`}</CodeBlock>
          <SectionLabel>{t('响应内容')}</SectionLabel>
          <p className="text-sm">
            {t('data.group 为更新后的素材组，data.results 为各线路上传结果；素材的 status 表示处理状态（Processing/Active/Failed），asset_id 为可用于 asset:// 引用的资产 ID。')}
          </p>
        </div>
      ),
    },
    {
      title: t('刷新与重命名'),
      subtitle: 'POST /refresh · PATCH /groups/{id} · PATCH /assets/{assetId}',
      content: (
        <div className="space-y-2">
          <EndpointHeader method="POST" path="/api/asset-library/groups/{id}/refresh" />
          <p className="text-sm">
            {t('拉取各上游最新素材状态并回写本地，用于轮询异步入库结果。')}
          </p>
          <SectionLabel>{t('重命名素材组')}</SectionLabel>
          <EndpointHeader method="PATCH" path="/api/asset-library/groups/{id}" />
          <p className="text-sm">
            {t('请求体：{ "display_name": "...", "description": "..." }，会同步到各上游。')}
          </p>
          <SectionLabel>{t('重命名素材')}</SectionLabel>
          <EndpointHeader method="PATCH" path="/api/asset-library/groups/{id}/assets/{assetId}" />
          <p className="text-sm">
            {t('请求体：{ "name": "..." }，会同步到各上游（OpenAI Files 上游不支持改名，将跳过）。')}
          </p>
        </div>
      ),
    },
    {
      title: t('删除素材与素材组'),
      subtitle: 'DELETE /assets/{assetId} · DELETE /groups/{id}',
      content: (
        <div className="space-y-2">
          <EndpointHeader method="DELETE" path="/api/asset-library/groups/{id}/assets/{assetId}" />
          <p className="text-sm">
            {t('从每个已映射上游删除该素材后再删除本地记录。')}
          </p>
          <SectionLabel>{t('删除素材组')}</SectionLabel>
          <EndpointHeader method="DELETE" path="/api/asset-library/groups/{id}" />
          <p className="text-sm">
            {t('素材组必须为空才能删除；组内仍有素材时返回错误，请先删除全部素材。')}
          </p>
          <Callout type="warn" title={t('删除不可恢复')}>
            {t('删除后无法再查询，也不能继续用于视频生成。需要保留时请先转存素材 URL。')}
          </Callout>
        </div>
      ),
    },
  ];

  const tutorialData = {
    volcengine: {
      description: t(
        '火山兼容 API 使用 Seedance 参数体系，通过顶层 content 数组传入文本与参考素材，采用「创建任务、轮询状态、读取结果」的异步模式，覆盖任务的创建、查询、列表、取消与删除。',
      ),
      steps: volcengineSteps,
    },
    openai: {
      description: t(
        'OpenAI 兼容 API 适合从 OpenAI Videos / Sora 客户端迁移，仅接受 application/json，通过顶层字段白名单传参，覆盖视频任务的创建、查询、内容下载与错误处理。',
      ),
      steps: openaiSteps,
    },
    assets: {
      description: t(
        '素材管理 API 为控制台接口（前缀 /api/asset-library，使用系统访问令牌鉴权），用于创建素材组、上传并轮询素材，随后在视频生成中以 asset://<ASSET_ID> 复用。',
      ),
      steps: assetSteps,
    },
  };

  const current = tutorialData[activeProtocol] || {};
  const currentSteps = current.steps || [];

  // 切换协议时，重置展开状态
  React.useEffect(() => {
    setOpenPanels([0]);
  }, [activeProtocol]);

  const getBackgroundColor = () =>
    actualTheme === 'dark' ? '#11111b' : '#ffffff';
  const getTextColor = () => (actualTheme === 'dark' ? '#e4e4e4' : '#1a1a1a');
  const getSecondaryTextColor = () =>
    actualTheme === 'dark' ? '#999' : '#666';

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
            {t('视频生成教程')}
          </h1>
          <p style={{ color: getSecondaryTextColor() }}>
            {t('提供火山兼容与 OpenAI 兼容两套接口，采用「创建任务、轮询状态、读取结果」的异步模式生成视频。')}
          </p>
          <p className="mt-1 text-sm" style={{ color: getSecondaryTextColor() }}>
            {t('服务地址：')}
            <InlineCode>{BASE_URL}</InlineCode>
            {t('，请求头统一使用 ')}
            <InlineCode>Authorization: Bearer &lt;API_KEY&gt;</InlineCode>
          </p>
        </div>

        {/* 协议选择标签 */}
        <div className="flex flex-wrap gap-2 mb-4">
          {protocols.map((p) => (
            <Button
              key={p.key}
              theme={activeProtocol === p.key ? 'solid' : 'borderless'}
              type={activeProtocol === p.key ? 'primary' : 'tertiary'}
              onClick={() => setActiveProtocol(p.key)}
              className="rounded-full px-6 py-2"
              style={{
                backgroundColor:
                  activeProtocol === p.key
                    ? '#f59e0b'
                    : actualTheme === 'dark'
                      ? '#2a2a3e'
                      : '#f0f0f0',
                borderColor:
                  activeProtocol === p.key
                    ? '#f59e0b'
                    : actualTheme === 'dark'
                      ? '#444'
                      : '#ddd',
                color: activeProtocol === p.key ? '#111' : getTextColor(),
              }}
            >
              {p.label}
            </Button>
          ))}
        </div>

        {/* 当前协议说明 */}
        <div className="mb-6 flex items-start gap-2">
          <Tag
            color={
              activeProtocol === 'volcengine'
                ? 'orange'
                : activeProtocol === 'assets'
                  ? 'green'
                  : 'blue'
            }
            size="small"
          >
            {activeProtocol === 'volcengine'
              ? t('Seedance 参数')
              : activeProtocol === 'assets'
                ? t('素材管理')
                : t('OpenAI 风格')}
          </Tag>
          <p className="text-sm flex-1" style={{ color: getSecondaryTextColor() }}>
            {current.description}
          </p>
        </div>

        {/* 接口卡片 */}
        <div className="space-y-2">
          {currentSteps.map((step, index) => (
            <CollapsiblePanel
              key={`${activeProtocol}-${index}`}
              index={index}
              title={step.title}
              subtitle={step.subtitle}
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

export default VideoTutorial;
