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

import React, {
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';
import { Button, Card, Empty, Spin, Typography } from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { IconRefresh } from '@douyinfe/semi-icons';
import { Gift } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../../context/User';
import { API, showError, showWarning } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import {
  getBlindBoxList,
  getEpayPaymentMethods,
  getMyBlindBoxOrders,
  purchaseBlindBox,
} from './api';
import BlindBoxCard from './components/BlindBoxCard';
import PurchaseModal from './components/PurchaseModal';
import RewardModal from './components/RewardModal';
import OrdersTable from './components/OrdersTable';

const { Text, Title } = Typography;

const TRADE_NO_STORAGE_KEY = 'blind_box_pending_trade_no';
const POLL_INTERVAL_MS = 2500;
const POLL_MAX_ATTEMPTS = 13;
const ORDERS_DEFAULT_PAGE_SIZE = 10;

// 只允许 http/https 的支付跳转地址，避免后端配置错误时打开危险协议；
// 同时兼容相对地址（按当前站点 origin 解析）。
function resolveSafeCheckoutUrl(value) {
  const trimmed = (value || '').trim();
  if (!trimmed) {
    return '';
  }
  try {
    const url = new URL(trimmed, window.location.origin);
    if (url.protocol === 'http:' || url.protocol === 'https:') {
      return url.href;
    }
  } catch (error) {
    return '';
  }
  return '';
}

// 参考 classic TopUp 页的易支付跳转实现
function submitEpayForm({ url, params }) {
  const form = document.createElement('form');
  form.action = url;
  form.method = 'POST';
  const isSafari =
    navigator.userAgent.indexOf('Safari') > -1 &&
    navigator.userAgent.indexOf('Chrome') < 1;
  if (!isSafari) {
    form.target = '_blank';
  }
  Object.keys(params || {}).forEach((key) => {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = key;
    input.value = params[key];
    form.appendChild(input);
  });
  document.body.appendChild(form);
  form.submit();
  document.body.removeChild(form);
}

const renderEmptyIllustration = () => ({
  image: <IllustrationNoResult style={{ width: 120, height: 120 }} />,
  darkModeImage: (
    <IllustrationNoResultDark style={{ width: 120, height: 120 }} />
  ),
});

const BlindBox = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [, userDispatch] = useContext(UserContext);

  const [activity, setActivity] = useState({
    enabled: false,
    cycle_id: 0,
    boxes: [],
  });
  const [boxesLoading, setBoxesLoading] = useState(true);
  const [loadError, setLoadError] = useState('');

  const [paymentMethods, setPaymentMethods] = useState([]);
  const [purchaseBox, setPurchaseBox] = useState(null);
  const [payMethod, setPayMethod] = useState('alipay');
  const [purchasing, setPurchasing] = useState(false);
  const [reward, setReward] = useState(null);

  const [orders, setOrders] = useState([]);
  const [ordersTotal, setOrdersTotal] = useState(0);
  const [ordersLoading, setOrdersLoading] = useState(true);
  const [ordersError, setOrdersError] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(ORDERS_DEFAULT_PAGE_SIZE);
  const [ordersRefreshToken, setOrdersRefreshToken] = useState(0);

  const loadBoxes = useCallback(
    async (options = {}) => {
      const { silent = false } = options;
      if (!silent) {
        setBoxesLoading(true);
      }
      try {
        const data = await getBlindBoxList();
        setActivity(data);
        setLoadError('');
      } catch (error) {
        if (!silent) {
          setLoadError(error?.message || t('加载失败'));
        }
      } finally {
        if (!silent) {
          setBoxesLoading(false);
        }
      }
    },
    [t],
  );

  const refreshUserQuota = useCallback(async () => {
    try {
      const res = await API.get('/api/user/self', { skipErrorHandler: true });
      if (res?.data?.success && res.data.data) {
        userDispatch({ type: 'login', payload: res.data.data });
      }
    } catch (error) {
      // 余额刷新失败不影响开奖结果展示
    }
  }, [userDispatch]);

  const handleRewardFound = useCallback(
    (order) => {
      setReward({
        boxName: order.box_name || '',
        multiplier: Number(order.drawn_multiplier) || 0,
        rewardQuota: Number(order.reward_quota) || 0,
      });
      setOrdersRefreshToken((value) => value + 1);
      loadBoxes({ silent: true });
      refreshUserQuota();
    },
    [loadBoxes, refreshUserQuota],
  );

  // 首次加载盲盒列表
  useEffect(() => {
    loadBoxes();
  }, [loadBoxes]);

  // 读取易支付可用的支付方式；失败时由页面回退到 alipay
  useEffect(() => {
    let cancelled = false;
    getEpayPaymentMethods()
      .then((methods) => {
        if (!cancelled) {
          setPaymentMethods(methods);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setPaymentMethods([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const availablePayMethods = useMemo(() => {
    if (paymentMethods.length > 0) {
      return paymentMethods;
    }
    return [{ type: 'alipay', name: t('支付宝') }];
  }, [paymentMethods, t]);

  // 支付方式异步加载后，保证当前选中项仍然有效
  useEffect(() => {
    if (!purchaseBox) return;
    if (!availablePayMethods.some((method) => method.type === payMethod)) {
      setPayMethod(availablePayMethods[0]?.type || 'alipay');
    }
  }, [availablePayMethods, purchaseBox, payMethod]);

  // 盲盒订单分页加载
  useEffect(() => {
    let cancelled = false;
    setOrdersLoading(true);
    getMyBlindBoxOrders(page, pageSize)
      .then((data) => {
        if (cancelled) return;
        setOrders(data.items);
        setOrdersTotal(data.total);
        setOrdersError('');
      })
      .catch((error) => {
        if (cancelled) return;
        setOrders([]);
        setOrdersTotal(0);
        setOrdersError(error?.message || t('加载失败'));
      })
      .finally(() => {
        if (!cancelled) {
          setOrdersLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [page, pageSize, ordersRefreshToken, t]);

  // 支付回跳开奖轮询：2.5 秒一次，最多约 30 秒；组件卸载时清理定时器
  useEffect(() => {
    const tradeNo = localStorage.getItem(TRADE_NO_STORAGE_KEY);
    if (!tradeNo) {
      return undefined;
    }

    let cancelled = false;
    let attempts = 0;
    let timerId = null;

    const poll = async () => {
      if (cancelled) return;
      attempts += 1;
      try {
        const data = await getMyBlindBoxOrders(1, 20);
        if (cancelled) return;

        const order = (data.items || []).find(
          (item) => item.trade_no === tradeNo,
        );
        if (order?.status === 'success') {
          localStorage.removeItem(TRADE_NO_STORAGE_KEY);
          handleRewardFound(order);
          return;
        }
        if (order?.status === 'failed') {
          localStorage.removeItem(TRADE_NO_STORAGE_KEY);
          showError(t('盲盒订单支付失败，请重新购买'));
          return;
        }
        if (order?.status === 'expired') {
          localStorage.removeItem(TRADE_NO_STORAGE_KEY);
          showWarning(t('盲盒订单已过期，请重新购买'));
          return;
        }
      } catch (error) {
        // 轮询期间忽略瞬时网络错误，继续重试
      }

      if (cancelled || attempts >= POLL_MAX_ATTEMPTS) {
        return;
      }
      timerId = setTimeout(poll, POLL_INTERVAL_MS);
    };

    timerId = setTimeout(poll, 0);
    return () => {
      cancelled = true;
      if (timerId) {
        clearTimeout(timerId);
      }
    };
  }, [handleRewardFound, t]);

  const visibleBoxes = useMemo(
    () => (activity.boxes || []).filter((box) => box && box.enabled !== false),
    [activity.boxes],
  );

  const openPurchase = (box) => {
    if (!box || !activity.enabled) return;
    setPayMethod(availablePayMethods[0]?.type || 'alipay');
    setPurchaseBox(box);
  };

  const closePurchase = () => {
    if (purchasing) return;
    setPurchaseBox(null);
  };

  const handlePurchase = async () => {
    if (!purchaseBox || purchasing) return;

    setPurchasing(true);
    try {
      const result = await purchaseBlindBox(purchaseBox.id, payMethod);

      // 在跳转支付前记录订单号，支付回跳后据此轮询开奖结果
      if (result.tradeNo) {
        localStorage.setItem(TRADE_NO_STORAGE_KEY, result.tradeNo);
      }

      const checkoutUrl = resolveSafeCheckoutUrl(result.url);
      if (!checkoutUrl) {
        showError(t('支付跳转地址无效，请联系管理员'));
        return;
      }

      setPurchaseBox(null);
      submitEpayForm({ url: checkoutUrl, params: result.params });
    } catch (error) {
      showError(error?.message || t('购买失败'));
    } finally {
      setPurchasing(false);
    }
  };

  const renderBoxSection = () => {
    if (loadError) {
      return (
        <Card className='!rounded-2xl shadow-sm border-0'>
          <Empty
            {...renderEmptyIllustration()}
            description={loadError}
            style={{ padding: 32 }}
          />
          <div className='text-center'>
            <Button onClick={() => loadBoxes()}>{t('重新加载')}</Button>
          </div>
        </Card>
      );
    }

    if (!activity.enabled) {
      return (
        <Card className='!rounded-2xl shadow-sm border-0'>
          <Empty
            {...renderEmptyIllustration()}
            description={t('盲盒活动未开启')}
            style={{ padding: 40 }}
          />
          <div
            className='pb-6 text-center text-sm'
            style={{ color: 'var(--semi-color-text-2)' }}
          >
            {t('活动开启后，可在此购买盲盒并查看开奖结果。')}
          </div>
        </Card>
      );
    }

    if (visibleBoxes.length === 0) {
      return (
        <Card className='!rounded-2xl shadow-sm border-0'>
          <Empty
            {...renderEmptyIllustration()}
            description={t('暂无可购买的盲盒')}
            style={{ padding: 40 }}
          />
        </Card>
      );
    }

    return (
      <div className='grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3'>
        {visibleBoxes.map((box) => (
          <BlindBoxCard
            key={box.id}
            box={box}
            activityEnabled={activity.enabled}
            onBuy={openPurchase}
          />
        ))}
      </div>
    );
  };

  return (
    <div className='mt-[60px] px-2'>
      <div className='mx-auto w-full max-w-7xl'>
        <Card
          className='!rounded-2xl shadow-sm border-0'
          bodyStyle={{ padding: isMobile ? 16 : 24 }}
        >
          <div className='flex items-start gap-3 sm:gap-4'>
            <div
              className='flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl'
              style={{
                background: 'var(--semi-color-primary-light-default)',
                color: 'var(--semi-color-primary)',
              }}
            >
              <Gift size={22} />
            </div>
            <div className='min-w-0'>
              <Title heading={4} style={{ marginBottom: 4 }}>
                {t('盲盒')}
              </Title>
              <Text type='secondary'>
                {t(
                  '购买盲盒，有机会获得更高余额奖励，奖励将直接发放到账户余额。',
                )}
              </Text>
            </div>
          </div>
        </Card>

        <div className='mt-4'>
          <Spin spinning={boxesLoading} style={{ width: '100%' }}>
            {renderBoxSection()}
          </Spin>
        </div>

        <Card
          className='!rounded-2xl shadow-sm border-0 mt-4'
          bodyStyle={{ padding: isMobile ? 14 : 24 }}
        >
          <div className='mb-4 flex flex-wrap items-center justify-between gap-3'>
            <div className='min-w-0'>
              <Title heading={5} style={{ marginBottom: 4 }}>
                {t('我的盲盒记录')}
              </Title>
              <Text type='secondary' size='small'>
                {t('支付成功后自动开奖，抽中倍率与获得余额会显示在这里。')}
              </Text>
            </div>
            <Button
              icon={<IconRefresh />}
              onClick={() => setOrdersRefreshToken((value) => value + 1)}
              loading={ordersLoading}
            >
              {t('刷新')}
            </Button>
          </div>

          <OrdersTable
            orders={orders}
            total={ordersTotal}
            loading={ordersLoading}
            page={page}
            pageSize={pageSize}
            onPageChange={(nextPage) => setPage(nextPage)}
            onPageSizeChange={(nextPageSize) => {
              setPageSize(nextPageSize);
              setPage(1);
            }}
            emptyText={ordersError || t('暂无盲盒记录')}
          />
        </Card>

        <PurchaseModal
          visible={Boolean(purchaseBox)}
          box={purchaseBox}
          payMethods={availablePayMethods}
          payMethod={payMethod}
          onPayMethodChange={setPayMethod}
          onConfirm={handlePurchase}
          onCancel={closePurchase}
          confirmLoading={purchasing}
        />

        <RewardModal
          visible={Boolean(reward)}
          reward={reward}
          onClose={() => setReward(null)}
        />
      </div>
    </div>
  );
};

export default BlindBox;
