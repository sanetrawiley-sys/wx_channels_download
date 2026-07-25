/**
 * @file 通用事件总线。
 *
 * 仅包含 mitt 实例和 DOM 生命周期事件。视频号专用事件通过
 * pkg/scraper/wxchannels/inject/channels.events.js 按需扩展 WXE。
 */
var WXE = (() => {
  var eventbus = mitt();
  var Events = {
    /** DOM 完全加载和解析 */
    DOMContentLoaded: "DOMContentLoaded",
    /** DOM 加载完成前 */
    DOMContentBeforeUnLoaded: "DOMContentBeforeUnLoaded",
    /** 所有资源加载完成 */
    WindowLoaded: "WindowLoaded",
    /** 页面卸载完成 */
    WindowUnLoaded: "WindowUnLoaded",
    /** 微信公众号刷新（由 wxmp scraper 使用） */
    OfficialAccountRefresh: "OfficialAccountRefresh",
  };
  return {
    Events: Events,
    emit: eventbus.emit,
    /** 通用 on / off，供扩展模块使用 */
    on: eventbus.on,
    off: eventbus.off,
    /** DOM 完全加载和解析 */
    onDOMContentLoaded(handler) {
      eventbus.on(Events.DOMContentLoaded, handler);
      return () => {
        eventbus.off(Events.DOMContentLoaded, handler);
      };
    },
    /** DOM 加载完成前 */
    onDOMContentBeforeUnLoaded(handler) {
      eventbus.on(Events.DOMContentBeforeUnLoaded, handler);
      return () => {
        eventbus.off(Events.DOMContentBeforeUnLoaded, handler);
      };
    },
    /** 所有资源加载完成 */
    onWindowLoaded(handler) {
      eventbus.on(Events.WindowLoaded, handler);
      return () => {
        eventbus.off(Events.WindowLoaded, handler);
      };
    },
    /** 页面卸载完成 */
    onWindowUnLoaded(handler) {
      eventbus.on(Events.WindowUnLoaded, handler);
      return () => {
        eventbus.off(Events.WindowUnLoaded, handler);
      };
    },
  };
})();

document.addEventListener("DOMContentLoaded", function () {
  WXE.emit(WXE.Events.DOMContentLoaded, {
    href: window.location.href,
  });
});
window.addEventListener("beforeunload", function () {
  // 用户即将离开页面时触发（DOM 还存在）
  WXE.emit(WXE.Events.DOMContentBeforeUnLoaded, {
    href: window.location.href,
  });
});
window.addEventListener("load", function () {
  WXE.emit(WXE.Events.WindowLoaded, {
    href: window.location.href,
  });
});
window.addEventListener("unload", function () {
  // 页面即将卸载时触发（DOM 即将被销毁）
  WXE.emit(WXE.Events.WindowUnLoaded, {
    href: window.location.href,
  });
});
