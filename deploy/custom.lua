function select(headers, endpoints)
    local uid = headers["x-uid"]
    if uid == nil then
        return -1
    end
    local hashed = 0
    for i = 1, #uid do
        hashed = hashed + string.byte(uid, i)
    end
    return hashed % #endpoints
end