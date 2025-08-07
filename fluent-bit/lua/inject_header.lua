function add_log_level_header(tag, timestamp, record)
    -- Récupère le niveau de log (level)
    local level = record["level"]
    if level ~= nil then
        -- Ajoute le champ X-Log-Level dans le record pour le header HTTP
        record["X-Log-Level"] = level
    else
        record["X-Log-Level"] = "INFO" -- valeur par défaut si level absent
    end
    return 1, timestamp, record
end