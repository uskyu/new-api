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

import React, { useContext, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Typography,
  Input,
  ScrollList,
  ScrollItem,
} from '@douyinfe/semi-ui';
import { API, showError, copy, showSuccess } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { API_ENDPOINTS } from '../../constants/common.constant';
import { StatusContext } from '../../context/Status';
import { useActualTheme } from '../../context/Theme';
import { marked } from 'marked';
import { useTranslation } from 'react-i18next';
import {
  IconGithubLogo,
  IconPlay,
  IconFile,
  IconCopy,
} from '@douyinfe/semi-icons';
import { Link } from 'react-router-dom';
import NoticeModal from '../../components/layout/NoticeModal';
import {
  OpenAI,
  Gemini,
  Suno,
  Midjourney,
  DeepSeek,
  Qwen,
  Claude,
  Minimax,
} from '@lobehub/icons';
import {
  Video,
  Image,
  Music,
  DollarSign,
  FlaskConical,
  PlugZap,
  Gauge,
  ShieldCheck,
  LifeBuoy,
} from 'lucide-react';

const { Text } = Typography;

const Home = () => {
  const { t, i18n } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const actualTheme = useActualTheme();
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(false);
  const [homePageContent, setHomePageContent] = useState('');
  const [noticeVisible, setNoticeVisible] = useState(false);
  const isMobile = useIsMobile();
  const isDemoSiteMode = statusState?.status?.demo_site_enabled || false;
  const docsLink = statusState?.status?.docs_link || '';
  const serverAddress =
    statusState?.status?.server_address || `${window.location.origin}`;
  const endpointItems = API_ENDPOINTS.map((e) => ({ value: e }));
  const [endpointIndex, setEndpointIndex] = useState(0);
  const isChinese = i18n.language.startsWith('zh');

  const modelCards = useMemo(
    () => [
      {
        title: 'Google Veo 3.1',
        desc: t('Google 视频模型，支持高质量镜头运动与稳定场景表达。'),
        tag: t('视频生成'),
        tagIcon: <Video size={14} />,
        logo: <Gemini.Color size={30} />,
      },
      {
        title: 'Runway Aleph',
        desc: t('面向复杂编辑任务的视频模型，可进行对象替换与风格控制。'),
        tag: t('视频生成'),
        tagIcon: <Video size={14} />,
        logo: <Minimax.Color size={30} />,
      },
      {
        title: 'Suno API',
        desc: t('高质量音乐生成能力，适用于歌词转旋律与多风格创作。'),
        tag: t('音乐生成'),
        tagIcon: <Music size={14} />,
        logo: <Suno size={30} />,
      },
      {
        title: '4o Image API',
        desc: t('面向图像生成与编辑，具备稳定细节表现和风格一致性。'),
        tag: t('图像生成'),
        tagIcon: <Image size={14} />,
        logo: <OpenAI size={30} />,
      },
      {
        title: 'Flux Kontext',
        desc: t('擅长高保真视觉生成，适用于产品图、海报与场景素材。'),
        tag: t('图像生成'),
        tagIcon: <Image size={14} />,
        logo: <DeepSeek.Color size={30} />,
      },
      {
        title: 'Nano Banana',
        desc: t('轻量图像模型，兼顾响应速度与稳定输出效果。'),
        tag: t('图像生成'),
        tagIcon: <Image size={14} />,
        logo: <Qwen.Color size={30} />,
      },
    ],
    [t],
  );

  const featureCards = useMemo(
    () => [
      {
        title: t('灵活计费，按量付费'),
        desc: t('点数体系透明，支持精细成本控制，适配团队从试验到生产。'),
        icon: <DollarSign size={26} />,
      },
      {
        title: t('Playground 免费试用'),
        desc: t('可在接入前快速验证模型效果，优化参数并沉淀调用模板。'),
        icon: <FlaskConical size={26} />,
      },
      {
        title: t('快速接入，文档完善'),
        desc: t('统一接口风格与示例，几分钟内完成从测试到上线的迁移。'),
        icon: <PlugZap size={26} />,
      },
      {
        title: t('高性能与可扩展性'),
        desc: t('支持高并发请求分发与多模型路由，满足持续增长的业务需求。'),
        icon: <Gauge size={26} />,
      },
      {
        title: t('企业级安全能力'),
        desc: t('支持密钥隔离、权限控制与审计日志，降低平台运行风险。'),
        icon: <ShieldCheck size={26} />,
      },
      {
        title: t('7x24 监控与支持'),
        desc: t('持续监控核心链路状态，保障关键服务稳定与可观测。'),
        icon: <LifeBuoy size={26} />,
      },
    ],
    [t],
  );

  const showcaseCards = useMemo(
    () => [
      {
        title: t('AI 视频生成 APIs'),
        desc: t(
          '通过统一接口接入视频生成模型，兼顾画面质量、速度与成本，适配营销、教育与媒体内容生产。',
        ),
        cta: t('获取密钥'),
        theme: 'video',
      },
      {
        title: t('AI 图像生成 APIs'),
        desc: t(
          '支持图像生成与编辑能力，适用于产品视觉、创意海报与高一致性素材输出。',
        ),
        cta: t('查看文档'),
        theme: 'image',
      },
      {
        title: t('AI 音乐生成 APIs'),
        desc: t(
          '以 API 方式批量生成音乐内容，支持旋律、结构与风格控制，适配创作与商业化应用。',
        ),
        cta: t('立即体验'),
        theme: 'music',
      },
    ],
    [t],
  );

  const heroStats = [
    { label: t('稳定性'), value: '99.9%' },
    { label: t('平均响应'), value: '24.6s' },
    { label: t('运维支持'), value: '24/7' },
    { label: t('安全评级'), value: '#1' },
  ];

  const displayHomePageContent = async () => {
    setHomePageContent(localStorage.getItem('home_page_content') || '');
    const res = await API.get('/api/home_page_content');
    const { success, message, data } = res.data;
    if (success) {
      let content = data;
      if (!data.startsWith('https://')) {
        content = marked.parse(data);
      }
      setHomePageContent(content);
      localStorage.setItem('home_page_content', content);

      if (data.startsWith('https://')) {
        const iframe = document.querySelector('iframe');
        if (iframe) {
          iframe.onload = () => {
            iframe.contentWindow.postMessage({ themeMode: actualTheme }, '*');
            iframe.contentWindow.postMessage({ lang: i18n.language }, '*');
          };
        }
      }
    } else {
      showError(message);
      setHomePageContent('加载首页内容失败...');
    }
    setHomePageContentLoaded(true);
  };

  const handleCopyBaseURL = async () => {
    const ok = await copy(serverAddress);
    if (ok) {
      showSuccess(t('已复制到剪切板'));
    }
  };

  useEffect(() => {
    const checkNoticeAndShow = async () => {
      const lastCloseDate = localStorage.getItem('notice_close_date');
      const today = new Date().toDateString();
      if (lastCloseDate !== today) {
        try {
          const res = await API.get('/api/notice');
          const { success, data } = res.data;
          if (success && data && data.trim() !== '') {
            setNoticeVisible(true);
          }
        } catch (error) {
          console.error('获取公告失败:', error);
        }
      }
    };

    checkNoticeAndShow();
  }, []);

  useEffect(() => {
    displayHomePageContent().then();
  }, []);

  useEffect(() => {
    const timer = setInterval(() => {
      setEndpointIndex((prev) => (prev + 1) % endpointItems.length);
    }, 3000);
    return () => clearInterval(timer);
  }, [endpointItems.length]);

  return (
    <div className='w-full overflow-x-hidden'>
      <NoticeModal
        visible={noticeVisible}
        onClose={() => setNoticeVisible(false)}
        isMobile={isMobile}
      />
      {homePageContentLoaded && homePageContent === '' ? (
        <div className='home-kie-layout'>
          <section className='home-kie-hero'>
            <div className='home-kie-orbit' />
            <div className='home-kie-hero-inner'>
              <h1
                className={`home-kie-hero-title ${isChinese ? 'home-kie-hero-title-cn' : ''}`}
              >
                {t('一套 API 覆盖视频、图像与音乐模型')}
              </h1>
              <p className='home-kie-hero-subtitle'>
                {t(
                  '统一接入主流多模态能力，降低接入成本并提升交付速度，让 AI 产品更快上线。',
                )}
              </p>

              <div className='home-kie-hero-actions'>
                <Link to='/console'>
                  <Button
                    theme='solid'
                    type='primary'
                    size={isMobile ? 'default' : 'large'}
                    className='home-kie-cta-primary'
                    icon={<IconPlay />}
                  >
                    {t('探索 AI API')}
                  </Button>
                </Link>
                {isDemoSiteMode && statusState?.status?.version ? (
                  <Button
                    size={isMobile ? 'default' : 'large'}
                    className='home-kie-cta-secondary'
                    icon={<IconGithubLogo />}
                    onClick={() =>
                      window.open(
                        'https://github.com/QuantumNous/new-api',
                        '_blank',
                      )
                    }
                  >
                    {statusState.status.version}
                  </Button>
                ) : (
                  docsLink && (
                    <Button
                      size={isMobile ? 'default' : 'large'}
                      className='home-kie-cta-secondary'
                      icon={<IconFile />}
                      onClick={() => window.open(docsLink, '_blank')}
                    >
                      {t('API 文档')}
                    </Button>
                  )
                )}
              </div>

              <div className='home-kie-endpoint-panel'>
                <Text className='home-kie-endpoint-label'>
                  {t('替换模型基址即可接入')}
                </Text>
                <Input
                  readonly
                  value={serverAddress}
                  size={isMobile ? 'default' : 'large'}
                  className='home-kie-endpoint-input'
                  suffix={
                    <div className='home-kie-endpoint-suffix'>
                      <ScrollList
                        bodyHeight={30}
                        style={{ border: 'unset', boxShadow: 'unset' }}
                      >
                        <ScrollItem
                          mode='wheel'
                          cycled={true}
                          list={endpointItems}
                          selectedIndex={endpointIndex}
                          onSelect={({ index }) => setEndpointIndex(index)}
                        />
                      </ScrollList>
                      <Button
                        type='primary'
                        onClick={handleCopyBaseURL}
                        icon={<IconCopy />}
                      />
                    </div>
                  }
                />
              </div>

              <div className='home-kie-stats'>
                {heroStats.map((item) => (
                  <div key={item.label} className='home-kie-stat-item'>
                    <div className='home-kie-stat-value'>{item.value}</div>
                    <div className='home-kie-stat-label'>{item.label}</div>
                  </div>
                ))}
              </div>
            </div>
          </section>

          <section className='home-kie-section'>
            <h2 className='home-kie-section-title'>
              {t('今日可接入的热门 AI 模型')}
            </h2>
            <div className='home-kie-model-grid'>
              {modelCards.map((card) => (
                <article key={card.title} className='home-kie-model-card'>
                  <div className='home-kie-model-head'>
                    <div className='home-kie-model-logo'>{card.logo}</div>
                    <span className='home-kie-model-tag'>
                      {card.tagIcon}
                      {card.tag}
                    </span>
                  </div>
                  <h3>{card.title}</h3>
                  <p>{card.desc}</p>
                </article>
              ))}
            </div>
          </section>

          <section className='home-kie-section'>
            <h2 className='home-kie-section-title'>
              {t('为什么选择 New API 进行 API 集成')}
            </h2>
            <div className='home-kie-feature-grid'>
              {featureCards.map((item) => (
                <article key={item.title} className='home-kie-feature-card'>
                  <div className='home-kie-feature-icon'>{item.icon}</div>
                  <h3>{item.title}</h3>
                  <p>{item.desc}</p>
                </article>
              ))}
            </div>
          </section>

          <section className='home-kie-section home-kie-showcases'>
            {showcaseCards.map((item, index) => (
              <div
                key={item.title}
                className={`home-kie-showcase-row ${index % 2 === 1 ? 'reverse' : ''}`}
              >
                <div className='home-kie-showcase-text'>
                  <h3>{item.title}</h3>
                  <p>{item.desc}</p>
                  <Button
                    type='primary'
                    size='large'
                    className='home-kie-showcase-btn'
                  >
                    {item.cta}
                  </Button>
                </div>
                <div className={`home-kie-showcase-media ${item.theme}`}>
                  <div className='home-kie-showcase-chip'>
                    {t('积分')} 8,051
                  </div>
                  <div className='home-kie-showcase-preview'>
                    {item.theme === 'video' && <Gemini.Color size={56} />}
                    {item.theme === 'image' && <OpenAI size={56} />}
                    {item.theme === 'music' && <Suno size={56} />}
                  </div>
                  <div className='home-kie-showcase-track'>
                    {item.theme === 'video' && t('视频生成工作台')}
                    {item.theme === 'image' && t('图像生成工作台')}
                    {item.theme === 'music' && t('音乐生成控制台')}
                  </div>
                  <div className='home-kie-showcase-progress'>
                    <span />
                    <span />
                    <span />
                  </div>
                </div>
              </div>
            ))}
          </section>

          <section className='home-kie-section home-kie-brand-strip'>
            <div className='home-kie-brand-item'>
              <Claude.Color size={24} />
              <span>Claude</span>
            </div>
            <div className='home-kie-brand-item'>
              <Midjourney size={24} />
              <span>Midjourney</span>
            </div>
            <div className='home-kie-brand-item'>
              <DeepSeek.Color size={24} />
              <span>DeepSeek</span>
            </div>
            <div className='home-kie-brand-item'>
              <Qwen.Color size={24} />
              <span>Qwen</span>
            </div>
          </section>
        </div>
      ) : (
        <div className='overflow-x-hidden w-full'>
          {homePageContent.startsWith('https://') ? (
            <iframe
              src={homePageContent}
              className='w-full h-screen border-none'
            />
          ) : (
            <div
              className='mt-[60px]'
              dangerouslySetInnerHTML={{ __html: homePageContent }}
            />
          )}
        </div>
      )}
    </div>
  );
};

export default Home;
