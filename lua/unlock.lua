-- 1.检查是否拥有锁
-- 2.删除
-- KEYS[1] 分布式锁的key
-- ARGV[1] 预期redis里面的value
if redis.call('get', KEYS[1]) == ARGV[1] then
    -- 是拥有锁
    return redis.call('del', KEYS[1])
else
    -- 不是自己拥有锁
    return 0
end