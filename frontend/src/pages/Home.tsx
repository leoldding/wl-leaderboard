import React, { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { FaLock } from "react-icons/fa";
import { apiJson } from "../api/client";
import type { LeaderboardRow } from "../types/leaderboard";

const Home: React.FC = () => {
    const [rows, setRows] = useState<LeaderboardRow[]>([]);

    useEffect(() => {
        const getRows = async () => {
            try {
                const rows = await apiJson<LeaderboardRow[]>("/api/v1/leaderboard");
                setRows(rows);
            } catch (error) {
                console.error(error);
            }
        }

        getRows();
    }, []);

    return (
        <div className="flex flex-col h-dvh overflow-hidden">
            <header className="py-8 z-50 shadow text-center text-2xl md:text-4xl bg-base-100">
                Weightlifting Leaderboard
            </header>
            <div className="card text-neutral-content w-full min-w-[80dvw] md:max-w-[calc(100dvw-64px)] mx-auto flex-1 min-h-0">
                <div className="card-body items-center text-center h-full min-h-0">
                    <div className="overflow-x-auto w-full rounded-box border border-base-content/5 bg-base-100 flex-1 min-h-0">
                        <table className="table table-zebra table-pin-rows text-center">
                            <thead className="font-bold text-md md:text-2xl">
                                <tr>
                                    <th>Name</th>
                                    <th>Snatch</th>
                                    <th>Clean & Jerk</th>
                                    <th>Total</th>
                                </tr>
                            </thead>
                            <tbody className="text-sm md:text-xl">
                                {rows.map((row) => (
                                    <tr key={row.id}>
                                        <td>{row.name}</td>
                                        <td>{row.snatch}</td>
                                        <td>{row.cleanjerk}</td>
                                        <td>{row.total}</td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </div>
            </div>
            <div className="flex justify-end p-6">
                <Link to="/admin">
                    <button className="flex items-center justify-center btn btn-lg btn-soft btn-accent btn-circle">
                        <FaLock />
                    </button>
                </Link>
            </div>
        </div>
    );
};

export default Home;



