function select(headers, endpoints)
    local shard = headers["x-uid"]
    if shard == nil then
        return -1
    end
    local shard_num = tonumber(string.sub(shard, -1))
    if shard_num == nil then
        return -2
    end
    return shard_num % #endpoints
end