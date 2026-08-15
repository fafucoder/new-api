import { API } from '../helpers';

export const listProxies = ({ page = 1, size = 20, keyword = '', status = 0 } = {}) =>
  API.get(
    `/api/proxy/?page=${page}&size=${size}&keyword=${encodeURIComponent(keyword)}&status=${status}`,
  );

export const getProxyOptions = ({ onlyEnabled = true } = {}) =>
  API.get(`/api/proxy/options?only_enabled=${onlyEnabled}`);

export const createProxy = (payload) => API.post('/api/proxy/', payload);

export const updateProxy = (payload) => API.put('/api/proxy/', payload);

export const deleteProxy = (id) => API.delete(`/api/proxy/${id}`);

export const testProxy = (id) => API.post(`/api/proxy/${id}/test`);

export const getProxyReferences = (id) => API.get(`/api/proxy/${id}/channels`);
