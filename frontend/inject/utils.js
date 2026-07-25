/**
 * @file 所有的工具函数 + API + 事件总线
 */
if (typeof WXEnv === "undefined") {
  throw new Error("env.js must be loaded before utils.js");
}
var WXU = (() => {
  var defaultRandomAlphabet =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  function __wx_uid__() {
    return random_string(12);
  }
  /**
   * 返回一个指定长度的随机字符串
   * @param length
   * @returns
   */
  function random_string(length) {
    return random_string_with_alphabet(length, defaultRandomAlphabet);
  }
  function random_string_with_alphabet(length, alphabet) {
    let b = new Array(length);
    let max = alphabet.length;
    for (let i = 0; i < b.length; i++) {
      let n = Math.floor(Math.random() * max);
      b[i] = alphabet[n];
    }
    return b.join("");
  }
  function sleep() {
    return new Promise((resolve) => {
      setTimeout(() => {
        resolve();
      }, 1000);
    });
  }
  function __wx_top_tip(text) {
    const tip = document.createElement("div");
    tip.className = "wx-top-tip";
    tip.textContent = text || "";
    document.body.appendChild(tip);
    setTimeout(() => {
      tip.remove();
    }, 3000);
    return {
      hide() {
        tip.remove();
      },
    };
  }
  function __wx_toast(text) {
    const toast = document.createElement("div");
    toast.className = "wx-toast";
    toast.textContent = text || "";
    document.body.appendChild(toast);
    setTimeout(() => {
      toast.remove();
    }, 2200);
    return {
      hide() {
        toast.remove();
      },
    };
  }
  function __wx_loading(text = "加载中") {
    const mask = document.createElement("div");
    mask.className = "wx-loading-mask";
    mask.innerHTML = `<div class="wx-loading-box"><span class="wx-loading-spinner"></span><span>${text}</span></div>`;
    document.body.appendChild(mask);
    return {
      hide() {
        mask.remove();
      },
    };
  }
  /**
   * @param {string} text
   */
  function __wx_copy(text) {
    var textArea = document.createElement("textarea");
    textArea.value = text;
    textArea.style.cssText = "position: absolute; top: -999px; left: -999px;";
    document.body.appendChild(textArea);
    textArea.select();
    document.execCommand("copy");
    document.body.removeChild(textArea);
  }
  /**
   * @param {LogMsg} params
   */
  function __wx_log(params) {
    console.log("[log]", params);
    fetch("/__wx_channels_api/tip", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(params),
    });
  }
  /**
   * @param {ErrorMsg} params
   */
  function __wx_error(params) {
    var _alert = params.alert ?? 1;
    fetch("/__wx_channels_api/error", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(params),
    });
    if (_alert) {
      __wx_top_tip(params.msg);
    }
  }
  const script_loaded_map = {};
  function __wx_load_script(src) {
    const existing = script_loaded_map[src];
    if (existing) {
      return existing;
    }
    const p = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.type = "text/javascript";
      script.src = src;
      script.onload = resolve;
      script.onerror = reject;
      document.head.appendChild(script);
    });
    script_loaded_map[src] = p;
    return p;
  }

  function remove_zero(num) {
    let result = Number(num);
    if (String(num).indexOf(".") > -1) {
      result = parseFloat(num.toString().replace(/0+?$/g, ""));
    }
    return result;
  }
  function bytes_to_size(bytes) {
    if (!bytes) {
      return "0KB";
    }
    if (bytes === 0) {
      return "0KB";
    }
    const symbols = ["bytes", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"];
    let exp = Math.floor(Math.log(bytes) / Math.log(1024));
    if (exp < 1) {
      return bytes + " " + symbols[0];
    }
    bytes = Number((bytes / Math.pow(1024, exp)).toFixed(2));
    const size = bytes;
    const unit = symbols[exp];
    if (Number.isInteger(size)) {
      return `${size}${unit}`;
    }
    return `${remove_zero(size.toFixed(2))}${unit}`;
  }
  function get_queries(href) {
    var [pathname, search] = decodeURIComponent(href).split("?");
    var queries = decodeURIComponent(search)
      .split("&")
      .map((item) => {
        const [key, value] = item.split("=");
        return {
          [key]: value,
        };
      })
      .reduce(
        (prev, cur) => ({
          ...prev,
          ...cur,
        }),
        {},
      );
    return queries;
  }

  /**
   * 支持回调的下载
   * @param {Response} response
   * @param {{ onStart: (v: { total_size: number }) => void, onProgress: (v: { loaded_size: number, progress: number | null }) => void, onEnd: (v: { blob: Blob }) => void }} handlers
   */
  async function download_with_progress(response, handlers) {
    var content_length = response.headers.get("Content-Length");
    var chunks = [];
    var total_size = content_length ? parseInt(content_length, 10) : 0;
    if (total_size) {
      if (handlers.onStart) {
        handlers.onStart({ total_size });
      }
    }
    var loaded_size = 0;
    var reader = response.body.getReader();
    while (true) {
      var { done, value } = await reader.read();
      if (done) {
        break;
      }
      chunks.push(value);
      loaded_size += value.length;
      if (handlers.onProgress) {
        handlers.onProgress({
          loaded_size,
          progress: total_size
            ? Number(((loaded_size / total_size) * 100).toFixed(2))
            : null,
        });
      }
    }
    var blob = new Blob(chunks);
    if (handlers.onEnd) {
      handlers.onEnd({ blob });
    }
    return blob;
  }

  /**
   * @param {RequestInfo | URL} url
   * @returns {Promise<[null, Response] | [Error, null]>}
   */
  async function __wx_fetch(url) {
    try {
      const r = await fetch(url);
      return [null, r];
    } catch (err) {
      return [/** @type {Error} */ (err), null];
    }
  }

  return {
    ...WXE,
    downloader: {
      show() {},
      hide() {},
      toggle() {},
      async create(feed, opt) {
        return [new Error("downloader not ready"), null];
      },
      async create_batch(feeds, opt) {
        return [new Error("downloader not ready"), null];
      },
    },
    /**
     * 类型转换相关
     */
    async media_buffer_to_wav(...args) {
      await __wx_load_script(__wx_asset_url("/lib/recorder.min.js"));
      return WXAudio.mediaBufferToWav(...args);
    },
    // wav_to_mp3_blob: WXAudio.wavBlobToMP3,
    async media_to_mp3(buf) {
      return WXAudio.mediaToMp3(buf);
    },
    /**  */
    sleep,
    resultify(fn) {
      return (...args) => {
        return new Promise((resolve) => {
          fn(...args)
            .then((data) => {
              resolve([null, data]);
            })
            .catch((err) => {
              resolve([err, null]);
            });
        });
      };
    },
    uid: __wx_uid__,
    bytes_to_size,
    remove_zero,
    parseJSON(v) {
      try {
        var r = JSON.parse(v);
        return [null, r];
      } catch (err) {
        return [err, null];
      }
    },
    load_script: __wx_load_script,
    download_with_progress,
    /**
     * @param {() => HTMLElement} selector
     * @returns
     */
    find_elm(selector) {
      return new Promise((resolve) => {
        var __count = 0;
        var __timer = setInterval(() => {
          __count += 1;
          var $elm = selector();
          if (!$elm) {
            if (__count >= 5) {
              clearInterval(__timer);
              __timer = null;
              resolve(null);
            }
            return;
          }
          resolve($elm);
          return;
        }, 200);
      });
    },
    get_queries,
    /**
     * 提示相关
     */
    copy: __wx_copy,
    log: __wx_log,
    error: __wx_error,
    loading() {
      return __wx_loading();
    },
    toast(text) {
      return __wx_toast(text);
    },
    // menu_item: __wx_menu_item,
    // create_dropdown_menu: __wx_create_dropdown_menu,
    // create_popover: __wx_create_popover,
    fetch: __wx_fetch,
    observe_node(selector, cb, error_cb) {
      var $existing = document.querySelector(selector);
      if ($existing) {
        cb($existing);
        return;
      }
      var timer = null;
      if (error_cb) {
        timer = setTimeout(() => {
          observer.disconnect();
          error_cb();
        }, 5000);
      }
      var observer = new MutationObserver((mutations, obs) => {
        mutations.forEach((mutation) => {
          if (mutation.type === "childList") {
            mutation.addedNodes.forEach((node) => {
              if (node.nodeType === 1) {
                if (node.matches(selector) || node.querySelector(selector)) {
                  clearTimeout(timer);
                  cb(
                    node.matches(selector)
                      ? node
                      : node.querySelector(selector),
                  );
                  if (document.querySelector(selector)) {
                    obs.disconnect();
                  }
                }
              }
            });
          }
        });
      });
      WXU.onWindowLoaded(() => {
        const $root = document.getElementById("app");
        if (!$root) {
          return;
        }
        observer.observe($root, {
          childList: true,
          subtree: true,
        });
      });
    },
    /**
     * @param {{ url: string; method: 'GET' | 'POST'; body?: any }} opt
     */
    async request(opt) {
      return new Promise((resolve, reject) => {
        var xhr = new XMLHttpRequest();
        xhr.open(opt.method, opt.url);
        xhr.setRequestHeader("Content-Type", "application/json");
        xhr.onload = async function () {
          // console.log("[request]xhr.responseText", xhr.responseText);
          try {
            var data = JSON.parse(xhr.responseText);
            if (data.code !== 0) {
              const err = new Error(data.msg);
              err.code = data.code;
              err.data = data.data;
              err.response = data;
              resolve([err, null]);
              return;
            }
            resolve([null, data.data]);
          } catch (e) {
            // ignore
          }
          resolve([null, xhr.responseText]);
        };
        xhr.onerror = function (err) {
          // console.log("[request]xhr.onerror", err);
          resolve([new Error(err.type), null]);
        };
        xhr.send(JSON.stringify(opt.body));
      });
    },
    async save(blob, filename) {
      await __wx_load_script(__wx_asset_url("/lib/FileSaver.min.js"));
      saveAs(blob, filename);
    },
    async Zip() {
      await __wx_load_script(__wx_asset_url("/lib/jszip.min.js"));
      const zip = new JSZip();
      return zip;
    },
    /**
     * @returns {ChannelsConfig}
     */
    get config() {
      return WXEnv.config;
    },
    env: {
      get isChannels() {
        return WXEnv.isChannels;
      },
      get isWxwork() {
        return WXEnv.isWxwork;
      },
    },
  };
})();

if (typeof window.Timeless !== "undefined") {
  const timeless = window.Timeless;
  Object.assign(timeless, timeless.kit);
  Object.assign(timeless, timeless.headless);
  // Rendering
  window.h = timeless.h;
  window.View = timeless.View;
  window.Fragment = timeless.Fragment;
  // Control flow
  window.Show = timeless.Show;
  window.For = timeless.For;
  window.Switch = timeless.Switch;
  window.Match = timeless.Match;
  // Reactivity
  window.ref = timeless.ref;
  window.refobj = timeless.refobj;
  window.refarr = timeless.refarr;
  window.computed = timeless.computed;
  window.combine = timeless.combine;
  window.isElement = timeless.isElement;
  // Styling
  window.cn = timeless.cn;
  window.classNames = timeless.classNames;
  // Primitives
  window.PopoverPrimitive = timeless.PopoverPrimitive;
  window.DropdownMenuPrimitive = timeless.DropdownMenuPrimitive;
  window.WaterfallPrimitive = timeless.WaterfallPrimitive;
  window.ScrollViewPrimitive = timeless.ScrollViewPrimitive;
  window.DialogPrimitive = timeless.DialogPrimitive;
  // SVG helpers
  window.SVG = timeless.SVG;
  window.Circle = timeless.Circle;
}
