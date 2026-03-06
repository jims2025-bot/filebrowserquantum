import { fetchURL } from "./utils";
import { notify } from "@/notify";  // Import notify for error handling
import { getApiPath } from "@/utils/url.js";

export default async function search(base, source, query, signal, limit = 100, offset = 0) {
  try {
    query = encodeURIComponent(query);
    if (!base.endsWith("/")) {
      base += "/";
    }
    const apiPath = getApiPath("api/search", { 
      scope: encodeURIComponent(base), 
      query: query, 
      source: source,
      limit: limit,
      offset: offset
    });
    const res = await fetchURL(apiPath, { signal });
    let data = await res.json();

    return data
  } catch (err) {
    if (err.name === 'AbortError') throw err;
    notify.showError(err.message || "Error occurred during search");
    throw err;
  }
}
