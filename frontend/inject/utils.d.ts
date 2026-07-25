type LogMsg = {
  /** 消息内容 */
  msg: string;
  /** 日志前缀，默认是 [FRONTEND] */
  prefix?: string;
  ignore_prefix?: 1;
  replace?: 1;
  end?: 1;
};
type ErrorMsg = {
  /** 是否同时调用 alert */
  alert?: number;
  /** 错误消息内容 */
  msg: string;
};