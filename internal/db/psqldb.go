package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v4"
)

/*
	This function has pre-defined queries which get called to create different tables and functions in the
	psql DB that the controller is connected to.

	params: DB connection for psql database
	Return:
		bool: returns true if the query passed successfully, false if it failed
		error: nil if there isnt any error, else appropriate error message will be displayed
*/
func CreateTablesAndRoutines(db *pgx.Conn) (bool, error) {

	var createTables string
	var reportFunc string

	createTables = `CREATE TABLE IF NOT EXISTS public.logs_0
	(
	  	report_period_start timestamp with time zone,
	  	report_period_end timestamp with time zone,
	  	interval_start timestamp with time zone,
	  	interval_end timestamp with time zone,
	  	namespace text,
	  	namespace_labels text
	);
	CREATE TABLE IF NOT EXISTS public.logs_1
	(
	   	report_period_start timestamp with time zone,
		report_period_end timestamp with time zone,
		interval_start timestamp with time zone,
		interval_end timestamp with time zone,
	  	node text,
	  	node_labels text
	);
	CREATE TABLE IF NOT EXISTS public.logs_2
	(
		report_period_start timestamp with time zone,
		report_period_end timestamp with time zone,
		interval_start timestamp with time zone,
		interval_end timestamp with time zone,
		node text,
		namespace text,
		pod text,
		pod_usage_cpu_core_seconds double precision,
		pod_request_cpu_core_seconds double precision,
		pod_limit_cpu_core_seconds double precision,
		pod_usage_memory_byte_seconds double precision,
		pod_request_memory_byte_seconds double precision,
		pod_limit_memory_byte_seconds double precision,
		node_capacity_cpu_cores double precision,
		node_capacity_cpu_core_seconds double precision,
		node_capacity_memory_bytes double precision,
		node_capacity_memory_byte_seconds double precision,
		resource_id text,
		pod_labels text
	);
	CREATE TABLE IF NOT EXISTS public.logs_3
	(
	  	report_period_start timestamp with time zone,
	  	report_period_end timestamp with time zone,
		interval_start timestamp with time zone,
		interval_end timestamp with time zone,
		namespace text,
		pod text,
		persistentvolumeclaim text,
		persistentvolume text,
		storageclass text,
		persistentvolumeclaim_capacity_bytes double precision,
		persistentvolumeclaim_capacity_byte_seconds double precision,
		volume_request_storage_byte_seconds double precision,
		persistentvolumeclaim_usage_byte_seconds double precision,
		persistentvolume_labels text,
		persistentvolumeclaim_labels text
	);
	CREATE TABLE IF NOT EXISTS public.history
	(
	  	file_names text,
	  	manifest jsonb,
	  	success boolean,
	  	crtime timestamp with time zone
	);
	CREATE TABLE IF NOT EXISTS public.reports_human(
		frequency text ,
		interval_start timestamp with time zone,
		interval_end timestamp with time zone,
		namespace text,
		"pods_avg_usage_cpu_core_total[millicore]" numeric,
		"pods_request_cpu_core_total[millicore]" numeric,
		"pods_limit_cpu_core_total[millicore]" numeric,
		"pods_avg_usage_memory_total[MB]" numeric,
		"pods_request_memory_total[MB]" numeric,
		"pods_limit_memory_total[MB]" numeric,
		"volume_storage_request_total[GB]" double precision,
		"persistent_volume_claim_capacity_total[GB]" double precision,
		"persistent_volume_claim_usage_total[GB]" double precision
	)`

	reportFunc = `CREATE OR REPLACE FUNCTION generate_report (frequency_ text)
		returns TABLE (
			frequency text ,
			interval_start timestamp with time zone,
			interval_end timestamp with time zone,
			namespace text,
			"pods_avg_usage_cpu_core_total[millicore]" numeric,
			"pods_request_cpu_core_total[millicore]" numeric,
			"pods_limit_cpu_core_total[millicore]" numeric,
			"pods_avg_usage_memory_total[MB]" numeric,
			"pods_request_memory_total[MB]" numeric,
			"pods_limit_memory_total[MB]" numeric,
			"persistent_volume_claim_usage_total[GB]" double precision,
			"volume_storage_request_total[GB]" double precision,
			"persistent_volume_claim_capacity_total[GB]" double precision
  
  		)
		as $$
		declare
	  	interval_start_date timestamp with time zone;
	 	interval_end_date timestamp with time zone;
	  	total_seconds double precision;
		begin
		if frequency_ = 'day' then
			interval_start_date := date_trunc('day', current_date at time zone 'UTC') - interval '24 hours';
			interval_end_date := date_trunc('day', current_date at time zone 'UTC');
			total_seconds := 24 * 3600;
		end if;
		if frequency_ = 'week' then
			interval_start_date := date_trunc('day', current_date at time zone 'UTC') - interval '7 days';
			interval_end_date := date_trunc('day', current_date at time zone 'UTC');
			total_seconds := 7 * 24 * 3600;
		end if;
		if frequency_ = 'month' then
			interval_start_date := date_trunc('day', current_date at time zone 'UTC') - interval '1 month';
			interval_end_date := date_trunc('day', current_date at time zone 'UTC');
			total_seconds := 30 * 24 * 3600;
		end if;
		RETURN QUERY EXECUTE '
			INSERT INTO
			reports_human (
				frequency,
				interval_start,
				interval_end,
				namespace,
				"pods_avg_usage_cpu_core_total[millicore]", -- granularity = minute
				"pods_request_cpu_core_total[millicore]", -- granularity = minute
				"pods_limit_cpu_core_total[millicore]", -- granularity = minute
				"pods_avg_usage_memory_total[MB]", -- granularity = minute
				"pods_request_memory_total[MB]", -- granularity = minute
				"pods_limit_memory_total[MB]", -- granularity = minute
				"volume_storage_request_total[GB]", -- granularity = minute
				"persistent_volume_claim_capacity_total[GB]", -- granularity = minute
				"persistent_volume_claim_usage_total[GB]" -- granularity = minute
			)
			SELECT
			' || quote_literal(frequency_) || ' as frequency,
			' || quote_literal(interval_start_date) || '::timestamp with time zone  as interval_start,
			' || quote_literal(interval_end_date) || '::timestamp with time zone  as interval_end,
			namespace,
			round(SUM(pod_avg_usage_cpu_core), 2),
			round(SUM(pod_request_cpu_core), 2),
			round(SUM(pod_limit_cpu_core), 2),
			round(SUM(pod_avg_usage_memory) / 1024 / 1024, 0),
			round(SUM(pod_request_memory) / 1024 / 1024, 0 ),
			round(SUM(pod_limit_memory) / 1024 / 1024, 0 ),
			round(SUM(volume_storage_request) / 1024 / 1024 / 1024, 2 ),
			round(SUM(persistent_volume_claim_usage) / 1024 / 1024 / 1024, 2 ),
			round(SUM(persistent_volume_claim_capacity) / 1024 / 1024 / 1024, 2 )
			FROM
			(SELECT
				namespace,
				(SUM(pod_usage_cpu_core_seconds) / ' || quote_literal(total_seconds) || ' * 1000 )::numeric      as pod_avg_usage_cpu_core,
				(MAX(pod_request_cpu_core_seconds) / 3600 * 1000 )::numeric   as pod_request_cpu_core,
				(MAX(pod_limit_cpu_core_seconds) / 3600 * 1000 )::numeric      as pod_limit_cpu_core,
				(SUM(pod_usage_memory_byte_seconds)   / ' || quote_literal(total_seconds) || ')::numeric as pod_avg_usage_memory,
				(MAX(pod_request_memory_byte_seconds) / 3600 )::numeric as pod_request_memory,
				(MAX(pod_limit_memory_byte_seconds)   / 3600 )::numeric as pod_limit_memory,
				(MAX(volume_request_storage_byte_seconds)   / 3600 )::numeric as volume_storage_request,
				(MAX(persistentvolumeclaim_capacity_byte_seconds)   / 3600 )::numeric as persistent_volume_claim_capacity,
				(SUM(persistentvolumeclaim_usage_byte_seconds)  / ' || quote_literal(total_seconds) || ')::numeric as persistent_volume_claim_usage
			FROM (SELECT namespace, pod, interval_start, pod_usage_cpu_core_seconds, pod_request_cpu_core_seconds, pod_limit_cpu_core_seconds,
					pod_usage_memory_byte_seconds, pod_request_memory_byte_seconds, pod_limit_memory_byte_seconds,
					0 AS volume_request_storage_byte_seconds, 0 AS persistentvolumeclaim_capacity_byte_seconds, 0 AS persistentvolumeclaim_usage_byte_seconds
					FROM logs_2
					UNION ALL
					SELECT namespace, pod, interval_start, 0.0 AS pod_usage_cpu_core_seconds, 0.0 AS pod_request_cpu_core_seconds,
					0.0 AS pod_limit_cpu_core_seconds, 0.0 AS pod_usage_memory_byte_seconds, 0.0 AS pod_request_memory_byte_seconds,
					0.0 AS pod_limit_memory_byte_seconds, volume_request_storage_byte_seconds, persistentvolumeclaim_capacity_byte_seconds,
					persistentvolumeclaim_usage_byte_seconds FROM logs_3) AS temp
			WHERE temp.interval_start >= ' || quote_literal(interval_start_date) || ' and temp.interval_start < ' || quote_literal(interval_end_date) || '
			GROUP BY (namespace, pod))
			AS t
			GROUP BY namespace returning *';
		end; $$ LANGUAGE plpgsql`

	err := createTablesHelper(db, createTables)
	if err != nil {
		return false, errors.New("Unable to create tables")
	}

	err = createTablesHelper(db, reportFunc)
	if err != nil {
		return false, errors.New("Unable to execute the function")
	}
	return true, nil
}

func createTablesHelper(db *pgx.Conn, queryString string) error {
	_, err := db.Exec(context.Background(), queryString)
	if err != nil {
		return errors.New("Unable to execute the psql command")
	}
	return nil
}
