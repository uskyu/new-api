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

import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import SkeletonWrapper from '../components/SkeletonWrapper';

const Navigation = ({
  mainNavLinks,
  isMobile,
  isLoading,
  userState,
  pricingRequireAuth,
  isPublicHeader,
}) => {
  const location = useLocation();

  const isLinkActive = (link) => {
    if (link.isExternal) {
      return false;
    }
    if (link.to === '/') {
      return location.pathname === '/';
    }
    return location.pathname.startsWith(link.to);
  };

  const renderNavLinks = () => {
    const baseClasses =
      'flex-shrink-0 flex items-center gap-1 font-semibold transition-all duration-200 ease-in-out';
    const hoverClasses = isPublicHeader
      ? 'home-kie-header-nav-link'
      : 'rounded-md hover:text-semi-color-primary';
    const spacingClasses = isPublicHeader ? '' : isMobile ? 'p-1' : 'p-2';

    const commonLinkClasses = `${baseClasses} ${spacingClasses} ${hoverClasses}`;

    return mainNavLinks.map((link) => {
      const activeClassName =
        isPublicHeader && isLinkActive(link) ? 'home-kie-header-nav-link-active' : '';
      const linkContent = <span>{link.text}</span>;

      if (link.isExternal) {
        return (
          <a
            key={link.itemKey}
            href={link.externalLink}
            target='_blank'
            rel='noopener noreferrer'
            className={`${commonLinkClasses} ${activeClassName}`}
          >
            {linkContent}
          </a>
        );
      }

      let targetPath = link.to;
      if (link.itemKey === 'console' && !userState.user) {
        targetPath = '/login';
      }
      if (link.itemKey === 'pricing' && pricingRequireAuth && !userState.user) {
        targetPath = '/login';
      }

      return (
        <Link
          key={link.itemKey}
          to={targetPath}
          className={`${commonLinkClasses} ${activeClassName}`}
        >
          {linkContent}
        </Link>
      );
    });
  };

  return (
    <nav
      className={`flex flex-1 items-center whitespace-nowrap scrollbar-hide ${isPublicHeader ? 'home-kie-header-nav' : 'gap-1 lg:gap-2 mx-2 md:mx-4 overflow-x-auto'}`}
    >
      <SkeletonWrapper
        loading={isLoading}
        type='navigation'
        count={4}
        width={60}
        height={16}
        isMobile={isMobile}
      >
        <div
          className={
            isPublicHeader ? 'home-kie-header-nav-shell' : 'flex items-center gap-1 lg:gap-2'
          }
        >
          {renderNavLinks()}
        </div>
      </SkeletonWrapper>
    </nav>
  );
};

export default Navigation;
