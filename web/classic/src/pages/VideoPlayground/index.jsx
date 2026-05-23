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
import React, { useState, useEffect, useRef, useContext, useCallback } from 'react';
import { Layout, Card, Select, Typography, Button, Switch, InputNumber, Input, Chat, Toast, Modal } from '@douyinfe/semi-ui';
import { Settings, X, Film, Users, Sparkles, Clock, Monitor, Ratio, Music, Droplets, Image, Video, Mic } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  API, processModelsData, processGroupsData, selectFilter, renderGroupOption,
  getUserIdFromLocalStorage, getLogo, createMessage, createLoadingAssistantMessage,
  stringToColor, encodeToBase64,
} from '../../helpers';
import { API_ENDPOINTS, MESSAGE_ROLES, MESSAGE_STATUS } from '../../constants/playground.constants';
import { UserContext } from '../../context/User';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import FloatingButtons from '../../components/playground/FloatingButtons';
import CustomInputRender from '../../components/playground/CustomInputRender';
import MessageActions from '../../components/playground/MessageActions';
import { PlaygroundProvider } from '../../contexts/PlaygroundContext';

const { Text, Title } = Typography;
const RATIOS = ['16:9', '9:16', '1:1', '4:3', '3:4'];
const RESOLUTIONS = ['480p', '720p', '1080p'];

// 生成用户头像（与 Playground 一致）
const generateAvatarDataUrl = (username) => {
  if (!username) {
    return 'https://lf3-static.bytednsdoc.com/obj/eden-cn/ptlz_zlp/ljhwZthlaukjlkulzlp/docs-icon.png';
  }
  const firstLetter = username[0].toUpperCase();
  const bgColor = stringToColor(username);
  const svg = `
    <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32">
      <circle cx="16" cy="16" r="16" fill="${bgColor}" />
      <text x="50%" y="50%" dominant-baseline="central" text-anchor="middle" font-size="16" fill="#ffffff" font-family="sans-serif">${firstLetter}</text>
    </svg>
  `;
  return `data:image/svg+xml;base64,${encodeToBase64(svg)}`;
};

// ─── Left settings panel ──────────────────────────────────────────────────────
const VideoSettingsPanel = ({ config, setConfig, models, groups, styleState, onClose }) => {
  const { t } = useTranslation();
  const set = (key) => (val) => setConfig((c) => ({ ...c, [key]: val }));

  return (
    <Card className='h-full flex flex-col' bordered={false}
      bodyStyle={{ padding: styleState.isMobile ? 16 : 24, height: '100%', display: 'flex', flexDirection: 'column' }}>
      <div className='flex items-center justify-between mb-6 flex-shrink-0'>
        <div className='flex items-center'>
          <div className='w-10 h-10 rounded-full bg-gradient-to-r from-purple-500 to-pink-500 flex items-center justify-center mr-3'>
            <Settings size={20} className='text-white' />
          </div>
          <Title heading={5} className='mb-0'>{t('视频配置')}</Title>
        </div>
        {styleState.isMobile && (
          <Button icon={<X size={16} />} onClick={onClose} theme='borderless' type='tertiary' size='small' />
        )}
      </div>

      <div className='space-y-6 overflow-y-auto flex-1 pr-2 model-settings-scroll'>
        <div>
          <div className='flex items-center gap-2 mb-2'>
            <Users size={16} className='text-gray-500' />
            <Text strong className='text-sm'>{t('分组')}</Text>
          </div>
          <Select placeholder={t('请选择分组')} value={config.group} onChange={set('group')}
            optionList={groups} filter={selectFilter} autoClearSearchValue={false}
            selection renderOptionItem={renderGroupOption}
            style={{ width: '100%' }} dropdownStyle={{ width: '100%', maxWidth: '100%' }}
            className='!rounded-lg' />
        </div>
        <div>
          <div className='flex items-center gap-2 mb-2'>
            <Sparkles size={16} className='text-gray-500' />
            <Text strong className='text-sm'>{t('模型')}</Text>
          </div>
          <Select placeholder={t('请选择模型')} value={config.model} onChange={set('model')}
            optionList={models} filter={selectFilter} autoClearSearchValue={false}
            selection style={{ width: '100%' }} dropdownStyle={{ width: '100%', maxWidth: '100%' }}
            className='!rounded-lg' />
        </div>
        <div>
          <div className='flex items-center gap-2 mb-2'>
            <Clock size={16} className='text-gray-500' />
            <Text strong className='text-sm'>{t('时长（秒）')}</Text>
          </div>
          <InputNumber value={config.duration} onChange={set('duration')} min={3} max={60} style={{ width: '100%' }} />
        </div>
        <div>
          <div className='flex items-center gap-2 mb-2'>
            <Monitor size={16} className='text-gray-500' />
            <Text strong className='text-sm'>{t('分辨率')}</Text>
          </div>
          <Select value={config.resolution} onChange={set('resolution')}
            optionList={RESOLUTIONS.map((r) => ({ label: r, value: r }))} style={{ width: '100%' }} className='!rounded-lg' />
        </div>
        <div>
          <div className='flex items-center gap-2 mb-2'>
            <Ratio size={16} className='text-gray-500' />
            <Text strong className='text-sm'>{t('画面比例')}</Text>
          </div>
          <Select value={config.ratio} onChange={set('ratio')}
            optionList={RATIOS.map((r) => ({ label: r, value: r }))} style={{ width: '100%' }} className='!rounded-lg' />
        </div>
        <div className='flex items-center justify-between'>
          <div className='flex items-center gap-2'>
            <Music size={16} className='text-gray-500' />
            <Text strong className='text-sm'>{t('生成音频')}</Text>
          </div>
          <Switch checked={config.generateAudio} onChange={set('generateAudio')} checkedText={t('开')} uncheckedText={t('关')} size='small' />
        </div>
        <div className='flex items-center justify-between'>
          <div className='flex items-center gap-2'>
            <Droplets size={16} className='text-gray-500' />
            <Text strong className='text-sm'>{t('水印')}</Text>
          </div>
          <Switch checked={config.watermark} onChange={set('watermark')} checkedText={t('开')} uncheckedText={t('关')} size='small' />
        </div>
        <div>
          <div className='flex items-center gap-2 mb-2'>
            <Image size={16} className='text-gray-500' />
            <Text strong className='text-sm'>{t('首帧图片 URL（可选）')}</Text>
          </div>
          <Input placeholder='https://...' value={config.firstFrameImage}
            onChange={(val) => set('firstFrameImage')(val)} style={{ width: '100%' }} className='!rounded-lg' />
        </div>
        <div>
          <div className='flex items-center gap-2 mb-2'>
            <Image size={16} className='text-gray-500' />
            <Text strong className='text-sm'>{t('尾帧图片 URL（可选）')}</Text>
          </div>
          <Input placeholder='https://...' value={config.lastFrameImage}
            onChange={(val) => set('lastFrameImage')(val)} style={{ width: '100%' }} className='!rounded-lg' />
        </div>
        <div>
          <div className='flex items-center gap-2 mb-2'>
            <Video size={16} className='text-gray-500' />
            <Text strong className='text-sm'>{t('参考视频 URL（可选）')}</Text>
          </div>
          <Input placeholder='https://...' value={config.referenceVideo}
            onChange={(val) => set('referenceVideo')(val)} style={{ width: '100%' }} className='!rounded-lg' />
        </div>
        <div>
          <div className='flex items-center gap-2 mb-2'>
            <Mic size={16} className='text-gray-500' />
            <Text strong className='text-sm'>{t('参考音频 URL（可选）')}</Text>
          </div>
          <Input placeholder='https://...' value={config.referenceAudio}
            onChange={(val) => set('referenceAudio')(val)} style={{ width: '100%' }} className='!rounded-lg' />
        </div>
      </div>
    </Card>
  );
};

// ─── Right chat area ──────────────────────────────────────────────────────────
const VideoChatArea = ({
  messages, onSend, chatRef, styleState, roleInfo,
  renderChatBoxAction, onClear,
}) => {
  const { t } = useTranslation();

  const renderInputArea = useCallback((props) => <CustomInputRender {...props} />, []);

  const renderChatBoxContent = useCallback(({ message }) => {
    if (message.role === MESSAGE_ROLES.ASSISTANT) {
      if (message.status === MESSAGE_STATUS.LOADING) {
        return <Text type='tertiary'>{t('生成中...')}</Text>;
      }
      if (message.videoUrl) {
        return <video src={message.videoUrl} controls style={{ width: 300, height: 250 }} />;
      }
      if (message.content) {
        return <Text type={message.status === MESSAGE_STATUS.ERROR ? 'danger' : undefined}>{message.content}</Text>;
      }
    }
    return <Text>{typeof message.content === 'string' ? message.content : ''}</Text>;
  }, [t]);

  return (
    <Card className='h-full' bordered={false}
      bodyStyle={{ padding: 0, height: 'calc(100vh - 66px)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      {!styleState.isMobile && (
        <div className='px-6 py-4 bg-gradient-to-r from-purple-500 to-blue-500 rounded-t-2xl'>
          <div className='flex items-center gap-3'>
            <div className='w-10 h-10 rounded-full bg-white/20 backdrop-blur flex items-center justify-center'>
              <Film size={20} className='text-white' />
            </div>
            <div>
              <Title heading={5} className='!text-white mb-0'>{t('AI 视频生成')}</Title>
            </div>
          </div>
        </div>
      )}
      <div className='flex-1 overflow-hidden'>
        <Chat
          ref={chatRef}
          chatBoxRenderConfig={{
            renderChatBoxContent,
            renderChatBoxAction,
            renderChatBoxTitle: () => null,
          }}
          renderInputArea={renderInputArea}
          roleConfig={roleInfo}
          style={{ height: '100%', maxWidth: '100%', overflow: 'hidden' }}
          chats={messages}
          onMessageSend={onSend}
          showClearContext
          onClear={onClear}
          className='h-full'
          placeholder={t('描述你想生成的视频内容...')}
        />
      </div>
    </Card>
  );
};

// ─── Page ─────────────────────────────────────────────────────────────────────
const VideoPlayground = () => {
  const { t } = useTranslation();
  const [userState] = useContext(UserContext);
  const isMobile = useIsMobile();
  const styleState = { isMobile };
  const chatRef = useRef(null);

  const [showSettings, setShowSettings] = useState(true);
  const [models, setModels] = useState([]);
  const [groups, setGroups] = useState([]);
  const [messages, setMessages] = useState([]);
  const [submitting, setSubmitting] = useState(false);
  const pollRef = useRef(null);
  // taskId -> messageId mapping for updating messages
  const taskMsgMap = useRef({});

  const [config, setConfig] = useState({
    model: '', group: '', duration: 5, resolution: '720p', ratio: '16:9',
    generateAudio: true, watermark: false,
    firstFrameImage: '', lastFrameImage: '', referenceVideo: '', referenceAudio: '',
  });

  // 角色信息（与 Playground 一致：用户使用基于用户名生成的头像）
  const roleInfo = {
    user: {
      name: userState?.user?.username || 'User',
      avatar: generateAvatarDataUrl(userState?.user?.username),
    },
    assistant: {
      name: t('AI 视频生成'),
      avatar: getLogo(),
    },
  };

  useEffect(() => {
    if (!userState?.user) return;
    API.get(API_ENDPOINTS.USER_MODELS).then((res) => {
      if (res?.data?.success) {
        const { modelOptions, selectedModel } = processModelsData(res.data.data, config.model);
        setModels(modelOptions);
        if (selectedModel !== config.model) setConfig((c) => ({ ...c, model: selectedModel }));
      }
    });
    API.get(API_ENDPOINTS.USER_GROUPS).then((res) => {
      if (res?.data?.success) {
        const userGroup = userState?.user?.group || JSON.parse(localStorage.getItem('user') || '{}')?.group;
        const opts = processGroupsData(res.data.data, userGroup);
        setGroups(opts);
        if (!config.group) setConfig((c) => ({ ...c, group: opts[0]?.value || '' }));
      }
    });
  }, [userState?.user]);

  const updateMessageById = useCallback((msgId, updates) => {
    setMessages((prev) => prev.map((m) => m.id === msgId ? { ...m, ...updates } : m));
  }, []);

  const pollTask = useCallback(async (taskId, msgId) => {
    const res = await API.get(`/api/task/self?p=1&page_size=50&start_timestamp=0&end_timestamp=9999999999`);
    if (!res?.data?.success) return;
    const task = (res.data.data?.items || []).find((t) => t.task_id === taskId);
    if (!task) return;

    if (task.status === 'SUCCESS') {
      updateMessageById(msgId, {
        status: MESSAGE_STATUS.COMPLETE,
        content: `${t('任务 ID')}: ${taskId}`,
        videoUrl: task.result_url || '',
      });
      delete taskMsgMap.current[taskId];
    } else if (task.status === 'FAILURE') {
      updateMessageById(msgId, {
        status: MESSAGE_STATUS.ERROR,
        content: task.fail_reason || t('生成失败'),
      });
      delete taskMsgMap.current[taskId];
    }
    // stop polling if no pending tasks
    if (Object.keys(taskMsgMap.current).length === 0) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, [t, updateMessageById]);

  const submitVideoTask = useCallback(async (content, loadingMsgId) => {
    try {
      const contentArr = [{ type: 'text', text: content }];
      if (config.firstFrameImage) contentArr.push({ type: 'image_url', image_url: { url: config.firstFrameImage }, role: 'reference_image' });
      if (config.lastFrameImage) contentArr.push({ type: 'image_url', image_url: { url: config.lastFrameImage }, role: 'reference_image' });
      if (config.referenceVideo) contentArr.push({ type: 'video_url', video_url: { url: config.referenceVideo }, role: 'reference_video' });
      if (config.referenceAudio) contentArr.push({ type: 'audio_url', audio_url: { url: config.referenceAudio }, role: 'reference_audio' });

      const metadata = {
        content: contentArr, generate_audio: config.generateAudio,
        ratio: config.ratio, duration: config.duration,
        watermark: config.watermark, resolution: config.resolution,
      };

      const formData = new FormData();
      formData.append('prompt', content);
      formData.append('model', config.model);
      formData.append('metadata', JSON.stringify(metadata));

      const res = await fetch('/api/v1/videos', {
        method: 'POST',
        headers: {
          'New-API-User': getUserIdFromLocalStorage(),
          ...(config.group ? { 'New-API-Group': config.group } : {}),
        },
        body: formData,
      });
      const rawText = await res.text();
      let data = {};
      if (rawText) {
        try {
          data = JSON.parse(rawText);
        } catch (_) {
          data = {};
        }
      }
      const taskId = data.id || data.task_id;
      if (taskId) {
        const prompts = JSON.parse(localStorage.getItem('video-task-prompts') || '{}');
        prompts[taskId] = content;
        localStorage.setItem('video-task-prompts', JSON.stringify(prompts));
        taskMsgMap.current[taskId] = loadingMsgId;
        if (!pollRef.current) {
          pollRef.current = setInterval(() => {
            Object.entries(taskMsgMap.current).forEach(([tid, mid]) => pollTask(tid, mid));
          }, 5000);
        }
      } else {
        const fallback = rawText && rawText.length < 500 ? rawText : `HTTP ${res.status}`;
        updateMessageById(loadingMsgId, {
          status: MESSAGE_STATUS.ERROR,
          content: data.message || data.error?.message || fallback || t('提交失败'),
        });
      }
    } catch (e) {
      updateMessageById(loadingMsgId, { status: MESSAGE_STATUS.ERROR, content: e.message });
    }
  }, [config, pollTask, updateMessageById, t]);

  const handleSend = useCallback(async (content) => {
    if (submitting) return;
    setSubmitting(true);

    const userMsg = createMessage(MESSAGE_ROLES.USER, content);
    const loadingMsg = createLoadingAssistantMessage();
    setMessages((prev) => [...prev, userMsg, loadingMsg]);

    await submitVideoTask(content, loadingMsg.id);
    setSubmitting(false);
  }, [submitting, submitVideoTask]);

  // 复制消息（与 Playground 完全一致）
  const handleMessageCopy = useCallback((targetMessage) => {
    const textToCopy = typeof targetMessage.content === 'string' ? targetMessage.content : '';
    if (!textToCopy) {
      Toast.warning({ content: t('此消息没有可复制的文本内容'), duration: 2 });
      return;
    }
    const copyToClipboard = async (text) => {
      if (navigator.clipboard?.writeText) {
        try {
          await navigator.clipboard.writeText(text);
          Toast.success({ content: t('消息已复制到剪贴板'), duration: 2 });
        } catch (err) {
          fallbackCopy(text);
        }
      } else {
        fallbackCopy(text);
      }
    };
    const fallbackCopy = (text) => {
      try {
        const textArea = document.createElement('textarea');
        textArea.value = text;
        textArea.style.cssText = 'position:fixed;top:-9999px;left:-9999px;opacity:0;pointer-events:none;z-index:-1;';
        textArea.setAttribute('readonly', '');
        document.body.appendChild(textArea);
        textArea.select();
        textArea.setSelectionRange(0, text.length);
        const successful = document.execCommand('copy');
        document.body.removeChild(textArea);
        if (successful) {
          Toast.success({ content: t('消息已复制到剪贴板'), duration: 2 });
        } else {
          throw new Error('execCommand copy failed');
        }
      } catch (err) {
        Toast.error({ content: t('复制失败，请手动选择文本复制'), duration: 4 });
      }
    };
    copyToClipboard(textToCopy);
  }, [t]);

  // 删除消息（与 Playground 完全一致：Modal.confirm 弹框）
  const handleMessageDelete = useCallback((targetMessage) => {
    Modal.confirm({
      title: t('确认删除'),
      content: t('确定要删除这条消息吗？'),
      okText: t('确定'),
      cancelText: t('取消'),
      okButtonProps: { type: 'danger' },
      onOk: () => {
        setMessages((prevMessages) => {
          let messageIndex = prevMessages.findIndex((msg) => msg === targetMessage);
          if (messageIndex === -1) {
            messageIndex = prevMessages.findIndex((msg) => msg.id === targetMessage.id);
          }
          if (messageIndex === -1) return prevMessages;

          let updatedMessages;
          if (targetMessage.role === MESSAGE_ROLES.USER && messageIndex < prevMessages.length - 1) {
            const nextMessage = prevMessages[messageIndex + 1];
            if (nextMessage.role === MESSAGE_ROLES.ASSISTANT) {
              Toast.success({ content: t('已删除消息及其回复'), duration: 2 });
              updatedMessages = prevMessages.filter(
                (_, index) => index !== messageIndex && index !== messageIndex + 1,
              );
            } else {
              Toast.success({ content: t('消息已删除'), duration: 2 });
              updatedMessages = prevMessages.filter((msg) => msg.id !== targetMessage.id);
            }
          } else {
            Toast.success({ content: t('消息已删除'), duration: 2 });
            updatedMessages = prevMessages.filter((msg) => msg.id !== targetMessage.id);
          }
          return updatedMessages;
        });
      },
    });
  }, [t]);

  // 重试（与 Playground 一致：删除目标消息及之后内容，并重新发送对应的用户提示）
  const handleMessageReset = useCallback((targetMessage) => {
    if (submitting) return;
    setMessages((prevMessages) => {
      let messageIndex = prevMessages.findIndex((msg) => msg === targetMessage);
      if (messageIndex === -1) {
        messageIndex = prevMessages.findIndex((msg) => msg.id === targetMessage.id);
      }
      if (messageIndex === -1) return prevMessages;

      let userMessage;
      let truncateIndex;
      if (targetMessage.role === MESSAGE_ROLES.USER) {
        userMessage = targetMessage;
        truncateIndex = messageIndex;
      } else {
        let i = messageIndex - 1;
        while (i >= 0 && prevMessages[i].role !== MESSAGE_ROLES.USER) i--;
        if (i < 0) return prevMessages;
        userMessage = prevMessages[i];
        truncateIndex = i;
      }

      const contentToSend = typeof userMessage.content === 'string' ? userMessage.content : '';
      if (!contentToSend) return prevMessages;

      const trimmed = prevMessages.slice(0, truncateIndex);
      const newUserMsg = createMessage(MESSAGE_ROLES.USER, contentToSend);
      const loadingMsg = createLoadingAssistantMessage();

      setTimeout(() => {
        submitVideoTask(contentToSend, loadingMsg.id);
      }, 100);

      return [...trimmed, newUserMsg, loadingMsg];
    });
  }, [submitting, submitVideoTask]);

  const handleClear = useCallback(() => setMessages([]), []);

  // 自定义消息操作区（重试/复制/删除）——去掉点赞/踩等默认操作，UI 与操练场一致
  const renderChatBoxAction = useCallback((props) => {
    const { message: currentMessage } = props;
    const isAnyMessageGenerating = messages.some(
      (msg) => msg.status === MESSAGE_STATUS.LOADING || msg.status === MESSAGE_STATUS.INCOMPLETE,
    );
    return (
      <MessageActions
        message={currentMessage}
        styleState={styleState}
        onMessageReset={handleMessageReset}
        onMessageCopy={handleMessageCopy}
        onMessageDelete={handleMessageDelete}
        isAnyMessageGenerating={isAnyMessageGenerating}
        showRoleToggle={false}
        showReset={false}
        showDelete={false}
      />
    );
  }, [messages, styleState, handleMessageReset, handleMessageCopy, handleMessageDelete]);

  useEffect(() => {
    API.get('/api/task/self?p=1&page_size=20&start_timestamp=0&end_timestamp=9999999999').then((res) => {
      if (!res?.data?.success) return;
      // 后端按 id desc 返回（最新在前）—— 反转成时间升序，最旧在最上、最新在最下。
      const items = (res.data.data?.items || []).slice().reverse();
      const prompts = JSON.parse(localStorage.getItem('video-task-prompts') || '{}');
      const msgs = [];
      items.forEach((task) => {
        const prompt = prompts[task.task_id] || task.properties?.input || '';
        // Semi Chat 按 createAt 排序，必须用 task 自身时间，
        // 否则历史消息全部用 Date.now() 会和当前会话的消息相互错位。
        const baseTs = (task.submit_time || task.created_at || 0) * 1000;
        msgs.push(createMessage(MESSAGE_ROLES.USER, prompt, { createAt: baseTs }));
        const assistantContent = task.status === 'SUCCESS'
          ? `${t('任务 ID')}: ${task.task_id}`
          : task.status === 'FAILURE'
            ? (task.fail_reason || t('生成失败'))
            : t('生成中...');
        const assistantStatus = task.status === 'SUCCESS'
          ? MESSAGE_STATUS.COMPLETE
          : task.status === 'FAILURE'
            ? MESSAGE_STATUS.ERROR
            : MESSAGE_STATUS.LOADING;
        const aMsg = createMessage(MESSAGE_ROLES.ASSISTANT, assistantContent, {
          status: assistantStatus,
          videoUrl: task.status === 'SUCCESS' ? (task.result_url || '') : '',
          createAt: baseTs + 1,
        });
        msgs.push(aMsg);
        if (assistantStatus === MESSAGE_STATUS.LOADING) {
          taskMsgMap.current[task.task_id] = aMsg.id;
        }
      });
      setMessages(msgs);
      if (Object.keys(taskMsgMap.current).length > 0) {
        pollRef.current = setInterval(() => {
          Object.entries(taskMsgMap.current).forEach(([tid, mid]) => pollTask(tid, mid));
        }, 5000);
      }
    });
  }, []);

  useEffect(() => () => clearInterval(pollRef.current), []);

  const playgroundCtx = { onPasteImage: () => {}, imageUrls: [], imageEnabled: false };

  return (
    <PlaygroundProvider value={playgroundCtx}>
      <div className='h-full'>
        <Layout className='h-full bg-transparent flex flex-col md:flex-row'>
          {(showSettings || !isMobile) && (
            <Layout.Sider
              className={`bg-transparent border-r-0 flex-shrink-0 overflow-auto mt-[60px] ${
                isMobile
                  ? 'fixed top-0 left-0 right-0 bottom-0 z-[1000] w-full h-auto bg-white shadow-lg'
                  : 'relative z-[1] w-80 h-[calc(100vh-66px)]'
              }`}
              width={isMobile ? '100%' : 320}
            >
              <VideoSettingsPanel
                config={config} setConfig={setConfig}
                models={models} groups={groups}
                styleState={styleState} onClose={() => setShowSettings(false)}
              />
            </Layout.Sider>
          )}

          <Layout.Content className='relative flex-1 overflow-hidden'>
            <div className='overflow-hidden flex flex-col lg:flex-row h-[calc(100vh-66px)] mt-[60px]'>
              <div className='flex-1 flex flex-col'>
                <VideoChatArea
                  messages={messages} onSend={handleSend}
                  chatRef={chatRef} styleState={styleState}
                  roleInfo={roleInfo}
                  renderChatBoxAction={renderChatBoxAction}
                  onClear={handleClear}
                />
              </div>
            </div>
            <FloatingButtons
              styleState={styleState} showSettings={showSettings} showDebugPanel={false}
              onToggleSettings={() => setShowSettings(!showSettings)} onToggleDebugPanel={() => {}}
            />
          </Layout.Content>
        </Layout>
      </div>
    </PlaygroundProvider>
  );
};

export default VideoPlayground;
