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
import { Button, Modal } from '@douyinfe/semi-ui';
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
  ArrowRight,
  Check,
  Gauge,
  LifeBuoy,
  PlugZap,
  ShieldCheck,
} from 'lucide-react';

const Home = () => {
  const { t, i18n } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const actualTheme = useActualTheme();
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(false);
  const [homePageContent, setHomePageContent] = useState('');
  const [noticeVisible, setNoticeVisible] = useState(false);
  const [contactVisible, setContactVisible] = useState(false);
  const isMobile = useIsMobile();
  const isDemoSiteMode = statusState?.status?.demo_site_enabled || false;
  const docsLink = statusState?.status?.docs_link || '';
  const serverAddress =
    statusState?.status?.server_address || `${window.location.origin}`;
  const endpointItems = API_ENDPOINTS.length
    ? API_ENDPOINTS
    : ['/v1/chat/completions'];
  const [endpointIndex, setEndpointIndex] = useState(0);
  const isChinese = i18n.language.startsWith('zh');

  const heroMetrics = useMemo(
    () => [
      {
        label: t('稳定性'),
        value: '99.9%',
      },
      {
        label: t('平均响应'),
        value: '24ms',
      },
      {
        label: t('运维支持'),
        value: '24/7',
      },
      {
        label: t('安全评级'),
        value: 'ISO 27001',
      },
    ],
    [t],
  );

  const coreCards = useMemo(
    () => [
      {
        title: t('快速接入，文档完善'),
        desc: t('统一接口风格与示例，几分钟内完成从测试到上线的迁移。'),
        icon: <PlugZap size={24} />,
      },
      {
        title: t('高性能与可扩展性'),
        desc: t('支持高并发请求分发与多模型路由，满足持续增长的业务需求。'),
        icon: <Gauge size={24} />,
      },
      {
        title: t('企业级安全能力'),
        desc: t('支持密钥隔离、权限控制与审计日志，降低平台运行风险。'),
        icon: <ShieldCheck size={24} />,
      },
      {
        title: t('7x24 监控与支持'),
        desc: t('持续监控核心链路状态，保障关键服务稳定与可观测。'),
        icon: <LifeBuoy size={24} />,
      },
    ],
    [t],
  );

  const advantageCards = useMemo(
    () => [
      [
        t('点数体系透明，支持精细成本控制，适配团队从试验到生产。'),
        t('可在接入前快速验证模型效果，优化参数并沉淀调用模板。'),
        t('统一接口风格与示例，几分钟内完成从测试到上线的迁移。'),
      ],
      [
        t('支持高并发请求分发与多模型路由，满足持续增长的业务需求。'),
        t('支持密钥隔离、权限控制与审计日志，降低平台运行风险。'),
        t('持续监控核心链路状态，保障关键服务稳定与可观测。'),
      ],
      [
        t('Google 视频模型，支持高质量镜头运动与稳定场景表达。'),
        t('面向图像生成与编辑，具备稳定细节表现和风格一致性。'),
        t('高质量音乐生成能力，适用于歌词转旋律与多风格创作。'),
      ],
    ],
    [t],
  );

  const rotatingEndpoint = endpointItems[endpointIndex] || endpointItems[0];
  const contactChannelLabel = '***';
  const contactIdPlaceholder = 'wx_fake_contact_001';

  const updateInteractiveCard = (cardNode, clientX, clientY) => {
    if (!cardNode) {
      return;
    }
    const rect = cardNode.getBoundingClientRect();
    const relX = (clientX - rect.left) / rect.width;
    const relY = (clientY - rect.top) / rect.height;
    const rotateY = (relX - 0.5) * 7;
    const rotateX = (0.5 - relY) * 7;
    const moveX = (relX - 0.5) * 6;
    const moveY = (relY - 0.5) * 6;
    cardNode.style.setProperty('--card-rotate-x', `${rotateX.toFixed(2)}deg`);
    cardNode.style.setProperty('--card-rotate-y', `${rotateY.toFixed(2)}deg`);
    cardNode.style.setProperty('--card-translate-x', `${moveX.toFixed(2)}px`);
    cardNode.style.setProperty('--card-translate-y', `${moveY.toFixed(2)}px`);
    cardNode.style.setProperty('--card-glow-x', `${(relX * 100).toFixed(2)}%`);
    cardNode.style.setProperty('--card-glow-y', `${(relY * 100).toFixed(2)}%`);
    cardNode.style.setProperty('--card-glow-opacity', '1');
  };

  const resetInteractiveCard = (cardNode) => {
    if (!cardNode) {
      return;
    }
    cardNode.style.setProperty('--card-rotate-x', '0deg');
    cardNode.style.setProperty('--card-rotate-y', '0deg');
    cardNode.style.setProperty('--card-translate-x', '0px');
    cardNode.style.setProperty('--card-translate-y', '0px');
    cardNode.style.setProperty('--card-glow-x', '50%');
    cardNode.style.setProperty('--card-glow-y', '50%');
    cardNode.style.setProperty('--card-glow-opacity', '0');
  };

  const handleCardPointerMove = (event) => {
    if (
      isMobile ||
      (typeof window !== 'undefined' &&
        window.matchMedia &&
        window.matchMedia('(prefers-reduced-motion: reduce)').matches)
    ) {
      return;
    }
    updateInteractiveCard(event.currentTarget, event.clientX, event.clientY);
  };

  const handleCardPointerLeave = (event) => {
    resetInteractiveCard(event.currentTarget);
  };

  const interactiveCardProps = isMobile
    ? {}
    : {
        onMouseMove: handleCardPointerMove,
        onMouseLeave: handleCardPointerLeave,
      };

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

  const handleCopyWechatId = async () => {
    const ok = await copy(contactIdPlaceholder);
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
      <Modal
        title={t('联系我们')}
        visible={contactVisible}
        onCancel={() => setContactVisible(false)}
        footer={null}
        centered
        className='home-v2-contact-modal-wrap'
      >
        <div className='home-v2-contact-modal'>
          <div className='home-v2-contact-modal-tag'>{contactChannelLabel}</div>
          <div className='home-v2-contact-modal-id'>{contactIdPlaceholder}</div>
          <div className='home-v2-contact-modal-actions'>
            <Button className='home-v2-contact-copy-btn' onClick={handleCopyWechatId}>
              {t('复制')}
            </Button>
            <Button type='primary' onClick={() => setContactVisible(false)}>
              {t('关闭')}
            </Button>
          </div>
        </div>
      </Modal>
      {homePageContentLoaded && homePageContent === '' ? (
        <div className='home-kie-layout home-v2-layout'>
          <section className='home-v2-hero'>
            <div className='home-v2-hero-grid'>
              <div className='home-v2-hero-copy'>
                <div className='home-v2-hero-badge'>{t('企业级安全能力')}</div>
                <h1
                  className={`home-v2-hero-title ${isChinese ? 'home-v2-hero-title-cn' : ''}`}
                >
                  {t('一套 API 覆盖视频、图像与音乐模型')}
                </h1>
                <p className='home-v2-hero-subtitle'>
                  {t(
                    '统一接入主流多模态能力，降低接入成本并提升交付速度，让 AI 产品更快上线。',
                  )}
                </p>

                <div className='home-v2-hero-actions'>
                  <Link to='/console'>
                    <Button
                      theme='solid'
                      type='primary'
                      size={isMobile ? 'default' : 'large'}
                      className='home-v2-cta-primary'
                      icon={<IconPlay />}
                    >
                      {t('立即体验')}
                    </Button>
                  </Link>
                  {isDemoSiteMode && statusState?.status?.version ? (
                    <Button
                      size={isMobile ? 'default' : 'large'}
                      className='home-v2-cta-secondary'
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
                    <Button
                      size={isMobile ? 'default' : 'large'}
                      className='home-v2-cta-secondary'
                      icon={<IconFile />}
                      onClick={() => {
                        if (docsLink) {
                          window.open(docsLink, '_blank');
                          return;
                        }
                        window.open('/pricing', '_self');
                      }}
                    >
                      {t('API 文档')}
                    </Button>
                  )}
                </div>

                <div className='home-v2-trusted'>
                  <span>OPENAI</span>
                  <span>ANTHROPIC</span>
                  <span>GOOGLE</span>
                  <span>GROK</span>
                </div>
              </div>

              <div
                className='home-v2-console home-v2-glass-card home-v2-interactive-card'
                {...interactiveCardProps}
              >
                <div className='home-v2-console-head'>
                  <div className='home-v2-console-dots'>
                    <span />
                    <span />
                    <span />
                  </div>
                  <div className='home-v2-console-chip home-v2-console-chip-bounce'>
                    ISO 27001
                  </div>
                </div>
                <div className='home-v2-console-uptime'>
                  <div className='home-v2-console-uptime-label'>{t('稳定性')}</div>
                  <div className='home-v2-console-uptime-value'>99.9%</div>
                  <div className='home-v2-console-progress'>
                    <span />
                  </div>
                </div>
                <div className='home-v2-console-grid'>
                  {heroMetrics.slice(1, 3).map((item) => (
                    <div key={item.label} className='home-v2-console-metric'>
                      <span>{item.label}</span>
                      <strong>{item.value}</strong>
                    </div>
                  ))}
                </div>
                <div className='home-v2-console-bottom'>
                  <div className='home-v2-console-bottom-icon'>
                    <ShieldCheck size={20} />
                  </div>
                  <div className='home-v2-console-bottom-text'>
                    <p>{t('企业级安全能力')}</p>
                    <span>{rotatingEndpoint}</span>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section className='home-kie-section home-v2-core'>
            <h2 className='home-v2-section-title'>
              {t('为什么选择 New API 进行 API 集成')}
            </h2>
            <div className='home-v2-core-grid'>
              {coreCards.map((item) => (
                <article
                  key={item.title}
                  className='home-v2-core-card home-v2-glass-card home-v2-interactive-card'
                  {...interactiveCardProps}
                >
                  <div className='home-v2-core-icon'>{item.icon}</div>
                  <h3>{item.title}</h3>
                  <p>{item.desc}</p>
                </article>
              ))}
            </div>
          </section>

          <section className='home-kie-section home-v2-advantages'>
            <h2 className='home-v2-section-title'>{t('今日可接入的热门 AI 模型')}</h2>
            <p className='home-v2-section-subtitle'>
              {t(
                '统一接入主流多模态能力，降低接入成本并提升交付速度，让 AI 产品更快上线。',
              )}
            </p>

            <div className='home-v2-adv-grid'>
              {advantageCards.map((items, index) => (
                <article
                  key={`adv-${index}`}
                  className='home-v2-adv-card home-v2-glass-card home-v2-interactive-card'
                  {...interactiveCardProps}
                >
                  {items.map((item) => (
                    <div key={item} className='home-v2-adv-item'>
                      <Check size={16} />
                      <span>{item}</span>
                    </div>
                  ))}
                </article>
              ))}

              <article
                className='home-v2-support-card home-v2-glass-card home-v2-interactive-card'
                {...interactiveCardProps}
              >
                <div className='home-v2-support-main'>
                  <div className='home-v2-support-icon'>
                    <LifeBuoy size={24} />
                  </div>
                  <div>
                    <h3>{t('7x24 监控与支持')}</h3>
                    <p>{t('持续监控核心链路状态，保障关键服务稳定与可观测。')}</p>
                  </div>
                </div>

                <div className='home-v2-support-copy'>
                  <span>{serverAddress}</span>
                  <Button
                    type='primary'
                    className='home-v2-copy-btn'
                    icon={<IconCopy />}
                    onClick={handleCopyBaseURL}
                  >
                    {t('复制')}
                  </Button>
                </div>
              </article>
            </div>
          </section>

          <section className='home-kie-section home-v2-contact'>
            <article className='home-v2-contact-card home-v2-glass-card'>
              <div className='home-v2-contact-copy'>
                <h3>{t('联系方式')}</h3>
                <p>{contactChannelLabel}</p>
              </div>
              <div className='home-v2-contact-actions'>
                <span className='home-v2-contact-id'>{contactIdPlaceholder}</span>
                <Button className='home-v2-contact-copy-btn' onClick={handleCopyWechatId}>
                  {t('复制')}
                </Button>
                <Button type='primary' onClick={() => setContactVisible(true)}>
                  {t('联系我们')}
                </Button>
              </div>
            </article>
          </section>

          <div className='home-v2-floating-actions'>
            <Button
              className='home-v2-float-btn home-v2-float-primary'
              icon={<LifeBuoy size={18} />}
              onClick={() => setContactVisible(true)}
            >
              {t('联系我们')}
            </Button>
            <Button
              className='home-v2-float-btn home-v2-float-secondary'
              icon={<ArrowRight size={18} />}
              onClick={() => {
                if (docsLink) {
                  window.open(docsLink, '_blank');
                  return;
                }
                window.open('/pricing', '_self');
              }}
            >
              {t('API 文档')}
            </Button>
          </div>
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
