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

import { useEffect, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { API } from '../../helpers';

const WIDGET_SCRIPT_PATH = 'design/defaulttheme/js/widgetv2/index.js';
const WIDGET_SCRIPT_FLAG = 'data-new-api-live-helper-chat';

function normalizeBaseUrl(value) {
  const trimmed = String(value || '').trim();
  if (!trimmed) return '';
  return trimmed.endsWith('/') ? trimmed : `${trimmed}/`;
}

const LiveHelperChatWidget = () => {
  const location = useLocation();
  const [config, setConfig] = useState(null);

  useEffect(() => {
    let cancelled = false;
    API.get('/api/chat_widget')
      .then((res) => {
        if (!cancelled && res?.data?.success) {
          setConfig(res.data.data);
        }
      })
      .catch(() => {
        // The public widget is optional; a failed config request must not
        // affect the rest of the page.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const eligible =
    location.pathname === '/' || location.pathname.startsWith('/console');
  const baseUrl =
    config?.enabled && config?.base_url
      ? normalizeBaseUrl(config.base_url)
      : '';

  useEffect(() => {
    if (!eligible || !baseUrl) return;
    if (window.__newApiLiveHelperChatBaseUrl) return;

    // LiveHelperChat's current embed expects the installation root as
    // `lhc_base_url` and then loads its widget runtime from that root. The
    // legacy `chat/getstatusembed` endpoint is not used by the v2 widget.
    window.__newApiLiveHelperChatBaseUrl = baseUrl;
    window.LHC_API = window.LHC_API || {};
    window.LHC_API.args = {
      ...(window.LHC_API.args || {}),
      mode: 'widget',
      lhc_base_url: baseUrl,
    };

    const script = document.createElement('script');
    script.type = 'text/javascript';
    script.async = true;
    script.crossOrigin = 'anonymous';
    script.src = `${baseUrl}${WIDGET_SCRIPT_PATH}?v=${Date.now()}`;
    script.setAttribute(WIDGET_SCRIPT_FLAG, 'true');
    document.head.appendChild(script);
  }, [eligible, baseUrl]);

  return null;
};

export default LiveHelperChatWidget;
