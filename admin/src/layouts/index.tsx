import { GithubFilled, InfoCircleFilled, QuestionCircleFilled, SmileFilled } from '@ant-design/icons';
import type { ProSettings } from '@ant-design/pro-components';
import * as Icons from '@ant-design/icons';
import { PageContainer, ProLayout, SettingDrawer, ProCard } from '@ant-design/pro-components';
import { useState, useEffect } from 'react';
import React from 'react';
import defaultProps from './_defaultProps';
import { useNavigate, useLocation, Outlet } from 'react-router-dom';
import { RouteObject } from '@/routers';
import { getMenuList } from '@/api/modules/user';
interface MenuItem {
  component: string;
  path: string;
  icon: React.ReactNode;
  name: string;
  children?: MenuItem[];
}
const getItem = (menuItem: RouteObject): MenuItem => {
  return {
    component: menuItem.element as string,
    path: menuItem.path || '',
    icon: addIcon(menuItem.icon || ''),
    name: menuItem.name || '',
  };
};
// 动态渲染 Icon 图标
const customIcons: { [key: string]: any } = Icons;
const addIcon = (name: string) => {
  // console.log(name, customIcons[name]);
  if (!customIcons[name]) return;
  return React.createElement(customIcons[name]);
};
const dealMenuList = (list: RouteObject[]): MenuItem[] => {
  if (!list) {
    return list;
  }
  return list.map((v: RouteObject) => {
    const menuItem: MenuItem = getItem(v);
    if (v?.children?.length) {
      menuItem.children = dealMenuList(v.children);
    }
    return menuItem;
  });
};
const Container: React.FC = (props: any) => {
  const { pathname: curPathname } = useLocation();
  const [settings, setSetting] = useState<Partial<ProSettings> | undefined>({
    layout: 'side',
  });
  // 设置选中菜单项
  const [pathname, setPathname] = useState(curPathname);
  const navigate = useNavigate();
  const layoutConfig = defaultProps;
  // 获取菜单列表并处理成 antd menu 需要的格式
  // 需要使用useState视图才会更新
  const [route, setRoute] = useState<any>({
    path: '/',
    routes: [],
  });
  const getMenuData = async (layoutConfig: any) => {
    layoutConfig.menu.loading = true;
    try {
      let { data } = await getMenuList();
      if (!data) return;
      data = [...data, ...defaultProps.route.routes];
      setRoute({
        path: '/',
        routes: dealMenuList(data),
      });
    } finally {
      layoutConfig.menu.loading = false;
    }
  };
  // 需要设置第二个参数依懒性，不然会无限循环
  useEffect(() => {
    getMenuData(layoutConfig);
  }, [layoutConfig]);
  return (
    <div
      id="app-pro-layout"
      style={{
        height: '100vh',
      }}
    >
      <ProLayout
        siderWidth={216}
        bgLayoutImgList={[
          {
            src: 'https://img.alicdn.com/imgextra/i2/O1CN01O4etvp1DvpFLKfuWq_!!6000000000279-2-tps-609-606.png',
            left: 85,
            bottom: 100,
            height: '303px',
          },
          {
            src: 'https://img.alicdn.com/imgextra/i2/O1CN01O4etvp1DvpFLKfuWq_!!6000000000279-2-tps-609-606.png',
            bottom: -68,
            right: -45,
            height: '303px',
          },
          {
            src: 'https://img.alicdn.com/imgextra/i3/O1CN018NxReL1shX85Yz6Cx_!!6000000005798-2-tps-884-496.png',
            bottom: 0,
            left: 0,
            width: '331px',
          },
        ]}
        {...layoutConfig}
        route={route}
        location={{
          pathname,
        }}
        avatarProps={{
          src: 'https://gw.alipayobjects.com/zos/antfincdn/efFD%24IOql2/weixintupian_20170331104822.jpg',
          title: '七妮妮',
          size: 'small',
        }}
        actionsRender={props => {
          if (props.isMobile) return [];
          return [
            <InfoCircleFilled key="InfoCircleFilled" />,
            <QuestionCircleFilled key="QuestionCircleFilled" />,
            <GithubFilled key="GithubFilled" />,
          ];
        }}
        menuItemRender={(item, dom) => (
          <div
            onClick={() => {
              setPathname(item.path || '/welcome');
              navigate(item.path || '/welcome');
            }}
          >
            {dom}
          </div>
        )}
        {...settings}
      >
        <PageContainer>
          <ProCard
            style={{
              height: '100vh',
              minHeight: 800,
            }}
          >
            {/* 类似vue-router的router-view */}
            <Outlet />
            <div />
          </ProCard>
        </PageContainer>
      </ProLayout>
      <SettingDrawer
        pathname={pathname}
        enableDarkTheme
        getContainer={() => document.getElementById('app-pro-layout')}
        settings={settings}
        onSettingChange={changeSetting => {
          setSetting(changeSetting);
        }}
        disableUrlParams={false}
      />
    </div>
  );
};
export default Container;